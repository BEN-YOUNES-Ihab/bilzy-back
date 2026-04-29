package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	stdhttp "net/http"
)

// ErrorCode is the machine-readable error code returned in the JSON body.
// The frontend's friendlyError() switches on this to pick a translation key,
// so do not rename without updating the client at the same time.
type ErrorCode string

const (
	CodeUnauthorized       ErrorCode = "unauthorized"
	CodeForbidden          ErrorCode = "forbidden"
	CodeNotFound           ErrorCode = "not_found"
	CodeValidation         ErrorCode = "validation"
	CodeDuplicate          ErrorCode = "duplicate"
	CodeInternal           ErrorCode = "internal"
	CodeInvalidCredentials ErrorCode = "invalid_credentials"
)

type APIError struct {
	Status  int       `json:"-"`
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *APIError) Error() string { return string(e.Code) + ": " + e.Message }

func NewError(status int, code ErrorCode, msg string) *APIError {
	return &APIError{Status: status, Code: code, Message: msg}
}

// Common shortcuts.
func ErrUnauthorized(msg string) *APIError { return NewError(stdhttp.StatusUnauthorized, CodeUnauthorized, msg) }
func ErrForbidden(msg string) *APIError    { return NewError(stdhttp.StatusForbidden, CodeForbidden, msg) }
func ErrNotFound(msg string) *APIError     { return NewError(stdhttp.StatusNotFound, CodeNotFound, msg) }
func ErrValidation(msg string) *APIError   { return NewError(stdhttp.StatusBadRequest, CodeValidation, msg) }
func ErrDuplicate(msg string) *APIError    { return NewError(stdhttp.StatusConflict, CodeDuplicate, msg) }
func ErrInternal(msg string) *APIError     { return NewError(stdhttp.StatusInternalServerError, CodeInternal, msg) }

// WriteJSON writes a JSON body with the given status. Logs and falls back to
// a plain 500 if encoding fails.
func WriteJSON(w stdhttp.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}

// WriteError converts any error into a JSON response. APIError shapes are
// used as-is; everything else is reported as a 500 with a generic message
// (and the original is logged).
func WriteError(w stdhttp.ResponseWriter, err error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		WriteJSON(w, apiErr.Status, apiErr)
		return
	}
	slog.Error("unhandled error", "err", err)
	WriteJSON(w, stdhttp.StatusInternalServerError, &APIError{
		Code:    CodeInternal,
		Message: "internal error",
	})
}
