package utils

import (
	"context"
	"encoding/json"
	"net/http"

	"golang.org/x/sync/errgroup" // Import errgroup
)

func RespondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, ErrorResponse{Success: false, Message: message})
}

// WaitGroupWithError simplifies running multiple goroutines and collecting the first error.
type WaitGroupWithError struct {
	errgroup.Group
	ctx context.Context
}

// NewWaitGroupWithError creates a new WaitGroupWithError associated with a context.
func NewWaitGroupWithError(ctx context.Context) *WaitGroupWithError {
	// Create an error group that cancels the context if any goroutine returns an error
	g, derivedCtx := errgroup.WithContext(ctx)
	return &WaitGroupWithError{Group: *g, ctx: derivedCtx}
}

// Add starts a new goroutine within the group.
// The provided function 'f' will receive the group's derived context.
func (wg *WaitGroupWithError) Add(f func(innerCtx context.Context) error) {
	wg.Go(func() error {
		// Pass the derived context to the function
		return f(wg.ctx)
	})
}

// Wait blocks until all goroutines added via Add have completed,
// or until the first non-nil error is returned, or the context is cancelled.
// It returns the first error encountered.
func (wg *WaitGroupWithError) Wait(parentCtx context.Context) error {
	// We use the parent context here primarily for logging/tracing if needed,
	// the actual cancellation is handled by the errgroup's context.
	return wg.Group.Wait()
}
