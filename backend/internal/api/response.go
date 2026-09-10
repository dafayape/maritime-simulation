// Package api is the REST presentation layer for the dashboard (SRS §3).
package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Ceftkiki47/e-logbook-simulation/backend/internal/domain"
)

// envelope is the mandatory response shape (SRS §3):
// { "status": "success" | "error", "message": string, "data": ... }.
type envelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func writeJSON(w http.ResponseWriter, status int, body envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func respondOK(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusOK, envelope{Status: "success", Message: message, Data: data})
}

func respondCreated(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusCreated, envelope{Status: "success", Message: message, Data: data})
}

func respondError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, envelope{Status: "error", Message: message, Data: nil})
}

// respondDomainError maps domain sentinel errors onto HTTP semantics.
func respondDomainError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, domain.ErrValidation):
		respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, domain.ErrSessionInactive):
		respondError(w, http.StatusConflict, domain.ErrSessionInactive.Error())
	case errors.Is(err, domain.ErrSessionStillActive):
		respondError(w, http.StatusConflict, domain.ErrSessionStillActive.Error())
	default:
		log.Error("internal error", "error", err.Error())
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

// decodeBody parses a JSON request body with a hard size cap.
func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}
