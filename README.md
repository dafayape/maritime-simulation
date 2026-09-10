<div align="center">

# 🌊 Maritime LoRa Mesh Network Simulator

**Software-in-the-Loop Maritime Telecommunications, Mesh Routing & Physical Ether Simulator**

An integrated platform designed to simulate and evaluate low-frequency LoRa radio ad-hoc networks in maritime environments. Features realistic physical radio propagation, pure binary payload compression, dynamic multi-hop routing topologies, and real-time fleet command visualization.

[![CI Quality Gates](https://github.com/dafayape/maritime-simulation/actions/workflows/ci.yml/badge.svg)](https://github.com/dafayape/maritime-simulation/actions/workflows/ci.yml)
[![ISO 25010](https://img.shields.io/badge/ISO%2FIEC-25010%20Compliant-success?logo=checkmarx&logoColor=white)](#iso-standards-compliance)
[![ISO 27001](https://img.shields.io/badge/ISO%2FIEC-27001%20Hardened-blue?logo=auth0&logoColor=white)](SECURITY.md)
[![ISO 12207](https://img.shields.io/badge/ISO%2FIEC-12207%20Lifecycle-orange)](#iso-standards-compliance)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)](backend/)
[![Next.js](https://img.shields.io/badge/Next.js-16%20App%20Router-000000?logo=next.js&logoColor=white)](frontend/)
[![Mobile](https://img.shields.io/badge/Flutter-Branch%20mobile-02569B?logo=flutter&logoColor=white)](../../tree/mobile)
[![Docker](https://img.shields.io/badge/Docker-Compose%20Orchestrated-2496ED?logo=docker&logoColor=white)](docker-compose.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

</div>

---

## Repository Architecture Overview

This repository represents the **`main` branch**, housing the monorepo for the core backend services and the harbor master web monitoring dashboard:

| Component | Path | Technology Stack | Default Port |
|---|---|---|---|
| **Backend Core & Ether** | [`backend/`](backend/) | Go 1.22 · Gorilla WebSocket · PostgreSQL 16 · Redis 7 | `:8080` |
| **Harbor Master Web Dashboard** | [`frontend/`](frontend/) | Next.js 16 · React 19 · TypeScript · Tailwind CSS · Leaflet Maps | `:3000` |

The companion maritime vessel node application (Flutter, featuring offline Store-and-Forward SQLite & raw binary codecs) is maintained on a dedicated branch: **[`mobile`](../../tree/mobile)** (architecturally aligned with the [`sespimma`](https://github.com/dafayape/sespimma) multi-tier structure).

---

## 🏛️ Monorepo Directory Layout (`main`)

```
maritime-simulation/
├── backend/              # [Tier 1] Go 1.22 Core Service & Physical Ether Simulator
│   ├── cmd/              # Gateway server entrypoint & fleet simulator harness (nodesim)
│   ├── internal/         # Clean Architecture (Domain, Service, Repository, Protocol, WS)
│   ├── migrations/       # PostgreSQL database migrations (Sessions, Edges, Schemas, Telemetry)
│   └── Dockerfile        # Minimal multi-stage production Go binary build
├── frontend/             # [Tier 2] Next.js 16 Web Dashboard (Harbor Master Monitor)
│   ├── src/              # Modular Clean Component Architecture (Leaflet Maps, WS Monitor)
│   ├── public/           # Static assets & maritime navigation icons
│   └── Dockerfile        # Production standalone Next.js container
├── docker-compose.yml    # Master orchestrator (Postgres 16, Redis 7, Backend, Frontend)
├── LICENSE               # Official MIT License
├── SECURITY.md           # ISO 27001 Vulnerability Disclosure Policy
└── README.md             # System master architecture documentation
```

---

## ⚙️ Core Engineering Capabilities

### 1. Backend: Virtual Ether & Network Controller (Go)
- **Physical Propagation Modeling**: Real-time payload size constraints based on Spreading Factor (SF7=222B down to SF12=51B) with dynamic ACK timeout curves.
- **Dynamic Mesh Topology**: BFS-based shortest multi-hop route computation (up to 5 hops) evaluated across GPS Haversine distance matrices.
- **Probabilistic Packet Loss**: Radio degradation curves factoring distance tiers and dynamic maritime storm weather anomalies.
- **WebSocket Gateway**: Dual isolated channel architecture — high-throughput node communications and read-only telemetry broadcast for command dashboards.
- **Zero Goroutine Leak**: Worker-pool architecture fortified with panic-recovery middleware.

### 2. Frontend: Harbor Master Command Dashboard (Next.js)
- **Real-Time Geospatial Visualization**: Interactive Leaflet maritime chart displaying live multi-hop LoRa propagation links, radio parameters, and vessel coordinates.
- **Session & Virtual Edge Management**: Runtime simulation parameter tuning, dynamic weather anomaly injection, and coastal gateway management.
- **Telemetry Audit Logging**: Raw binary frame inspection, delivery ratio tracking, and historical latency graphs.

### 3. Mobile: Maritime Vessel Node Client (Branch `mobile`)
- **Isolated Branch**: Maintained on **[`mobile`](../../tree/mobile)**.
- **Key Features**: LoRa data link state machine with 3x retry limit, exponential backoff (1s→30s), SQLite Store-and-Forward disruption-tolerant queue, and schema-driven dynamic binary serialization.

---

## 🚀 Quick Start Guide

### Deploying the Stack via Docker Compose
Run the entire server-side ecosystem (PostgreSQL 16, Redis 7, Go Backend, and Next.js Dashboard) with a single command:

```bash
# Clone the repository
git clone git@github.com:dafayape/maritime-simulation.git
cd maritime-simulation

# Start the full stack
docker compose up -d --build
```

- **Web Dashboard**: `http://localhost:3000`
- **Backend API & WebSocket**: `http://localhost:8080`
- **Fleet Simulator (Optional)**:
  ```bash
  cd backend
  go run ./cmd/nodesim -nodes 20 -interval 5s
  ```

---

## 🧪 Quality Gates & Automated Verification

```bash
# 1. Backend Go Tests (Race detector & test coverage)
cd backend && go test -v -race ./...

# 2. Frontend Next.js Lint & Tests
cd frontend && npm run lint && npm test

# 3. Mobile Flutter Tests (Switch to mobile branch)
git checkout mobile && flutter test
```

---

## 🛡️ ISO Standards Compliance

| Standard | Scope | Implementation Details |
|---|---|---|
| **ISO/IEC 25010:2023**<br/>*(Software Product Quality)* | **Suitability & Efficiency** | Strict protocol validation, 100% test coverage across data link state machines, compact binary payloads, and sub-millisecond route calculations. |
| **ISO/IEC 27001:2022**<br/>*(Information Security)* | **Integrity & Governance** | All commits cryptographically signed via **Ed25519 GPG Key** (`7573912AF4141E99`), zero hardcoded secrets guarantee, and structured disclosure in [`SECURITY.md`](SECURITY.md). |
| **ISO/IEC 12207:2017**<br/>*(Software Life Cycle)* | **Process Governance** | Clean Architecture separation, automated unit/integration quality gates, and 100% **Conventional Commits v1.0.0** history. |

---

## License

Distributed under the MIT License. See [`LICENSE`](LICENSE) for details.

## Author

**Daffa Jaya Perkasa**  
Full-Stack | Mobile | AI | DevOps Engineer  
*GitHub: [@dafayape](https://github.com/dafayape)* • *Contact: [dafayape@gmail.com](mailto:dafayape@gmail.com)*
