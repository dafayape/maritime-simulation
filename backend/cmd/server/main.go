// Command server is the Maritime LoRa Mesh Network Simulator backend:
// REST API for the dashboard, WebSocket gateway for the virtual ships, mesh
// topology scheduler, and the probabilistic virtual ether.
package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/api"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/config"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/infra"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/repository"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/service"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/ws"
	"github.com/Ceftkiki47/e-logbook-simulation/backend/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		// The logger needs the config; fall back to stderr for config errors.
		os.Stderr.WriteString("config error: " + err.Error() + "\n")
		os.Exit(1)
	}
	log := infra.NewLogger(cfg.LogLevel)

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- infrastructure ---------------------------------------------------
	pool, err := infra.NewPostgresPool(rootCtx, cfg.PostgresDSN, log)
	if err != nil {
		log.Error("postgres init failed", "error", err.Error())
		os.Exit(1)
	}
	defer pool.Close()

	if err := infra.Migrate(rootCtx, pool, migrations.FS, log); err != nil {
		log.Error("migrations failed", "error", err.Error())
		os.Exit(1)
	}

	rdb, err := infra.NewRedisClient(rootCtx, cfg.RedisAddr, cfg.RedisPass, cfg.RedisDB, log)
	if err != nil {
		log.Error("redis init failed", "error", err.Error())
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()

	// --- repositories -----------------------------------------------------
	geoRepo := repository.NewGeoRepository(rdb)
	stateRepo := repository.NewStateRepository(rdb)
	routeRepo := repository.NewRouteRepository(rdb)
	sessionRepo := repository.NewSessionRepository(pool)
	edgeRepo := repository.NewEdgeRepository(pool)
	schemaRepo := repository.NewSchemaRepository(pool)
	telemetryRepo := repository.NewTelemetryRepository(pool)

	// --- transport hub + services -----------------------------------------
	hub := ws.NewHub(cfg.WSSendBuffer, log)

	anomaly := service.NewEnvironmentalAnomalyService()
	topology := service.NewMeshTopologyService(
		geoRepo, routeRepo, edgeRepo, stateRepo, hub, cfg.TopologyInterval, log)
	schemaSvc := service.NewDynamicSchemaService(schemaRepo, sessionRepo, hub, log)
	engine := service.NewSimulationEngineService(
		geoRepo, stateRepo, telemetryRepo, edgeRepo,
		schemaSvc, topology, anomaly, hub, cfg.PingMinInterval, log)
	sessionSvc := service.NewSessionService(
		sessionRepo, stateRepo, geoRepo, routeRepo, topology, hub, log)

	// Resume sessions that were active before a restart (VPS reboot safety).
	if err := sessionSvc.ResumeActive(rootCtx); err != nil {
		log.Error("resume active sessions failed", "error", err.Error())
		os.Exit(1)
	}

	// --- websocket worker pool + topology scheduler -------------------------
	gateway := ws.NewGateway(rootCtx, hub, engine, log)
	workers := cfg.WorkerPoolSize
	if workers <= 0 {
		workers = runtime.NumCPU() * 4
		if workers < 4 {
			workers = 4
		}
	}
	gateway.StartWorkers(workers)
	go topology.Run(rootCtx)

	// --- HTTP server --------------------------------------------------------
	readiness := func(ctx context.Context) error {
		if err := pool.Ping(ctx); err != nil {
			return err
		}
		return rdb.Ping(ctx).Err()
	}
	restServer := api.NewServer(
		sessionSvc, schemaSvc, topology, edgeRepo, telemetryRepo, readiness, log)
	handler := api.Router(restServer, api.WSHandlers{
		Nodes:   gateway.HandleNodeWS,
		Monitor: gateway.HandleMonitorWS,
	}, cfg.CORSAllowedOrigins, log)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if cfg.PprofAddr != "" {
		go servePprof(cfg.PprofAddr, log)
	}

	go func() {
		log.Info("server listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server failed", "error", err.Error())
			stop() // unblock main for cleanup
		}
	}()

	<-rootCtx.Done()
	log.Info("shutdown signal received, draining")

	shutCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		log.Warn("http shutdown incomplete", "error", err.Error())
	}
	hub.Shutdown()
	gateway.Wait()
	log.Info("goodbye")
}

// servePprof exposes the profiler on its own listener (keep it bound to
// localhost in production). Used to prove the >50-connection goroutine-leak
// acceptance criterion.
func servePprof(addr string, log interface{ Info(string, ...any) }) {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	log.Info("pprof listening", "addr", addr)
	srv := &http.Server{Addr: addr, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		log.Info("pprof server stopped", "error", err.Error())
	}
}
