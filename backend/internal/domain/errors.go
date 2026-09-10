// Package domain holds the hard invariants of the simulator: LoRa physical
// constraints, mesh routing rules, the artificial packet-loss model and the
// dynamic payload schema. It has zero infrastructure dependencies so every
// rule is unit-testable in isolation.
package domain

import "errors"

var (
	// ErrNotFound is returned by repositories when a row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrSessionInactive rejects operations against a stopped session.
	ErrSessionInactive = errors.New("simulation session is not active")
	// ErrSessionStillActive rejects destructive operations (deleting a
	// session's history) against a session that hasn't been stopped yet.
	ErrSessionStillActive = errors.New("simulation session is still active; stop it before deleting its history")
	// ErrValidation wraps user-input violations (HTTP 400 / drop reasons).
	ErrValidation = errors.New("validation error")
)
