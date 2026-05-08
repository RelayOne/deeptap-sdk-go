package deeptap

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// Error is the base type returned for any non-2xx response from the
// DeepTap API. Callers can switch on the typed sub-errors below or use
// errors.As to inspect the embedded fields directly.
type Error interface {
	error
	StatusCode() int
	ProblemType() string
	RequestID() string
}

// APIError carries the parsed RFC 7807 problem document and the
// underlying HTTP status. Every typed error in this package embeds it.
type APIError struct {
	Status     int
	Type       string
	Detail     string
	RequestIDV string
	Body       string
}

// StatusCode reports the HTTP status from the failing response.
func (e *APIError) StatusCode() int { return e.Status }

// ProblemType reports the RFC 7807 "type" field, or "unknown" if none.
func (e *APIError) ProblemType() string { return e.Type }

// RequestID reports the server-stamped request id, when present.
func (e *APIError) RequestID() string { return e.RequestIDV }

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("deeptap: api %d %s: %s", e.Status, e.Type, e.Detail)
}

// BadRequestError -- 400; caller-supplied input is invalid.
type BadRequestError struct{ APIError }

// AuthenticationError -- 401; the API key / DPoP token / mTLS cert was rejected.
type AuthenticationError struct{ APIError }

// PaymentRequiredError -- 402; payment required (x402 / MPP / out-of-credit).
type PaymentRequiredError struct{ APIError }

// NotFoundError -- 404.
type NotFoundError struct{ APIError }

// RateLimitedError -- 429; the rate-limit middleware capped the caller.
// RetryAfter mirrors the Retry-After header (seconds) when supplied.
type RateLimitedError struct {
	APIError
	RetryAfter time.Duration
}

// ServerError -- 5xx other than upstream-specific codes.
type ServerError struct{ APIError }

// UpstreamError -- 502 / 503 / 504.
type UpstreamError struct{ APIError }

// IsRateLimited reports whether err is a RateLimitedError.
func IsRateLimited(err error) bool {
	var rl *RateLimitedError
	return errors.As(err, &rl)
}

// IsAuthenticationError reports whether err is a 401 from the API.
func IsAuthenticationError(err error) bool {
	var ae *AuthenticationError
	return errors.As(err, &ae)
}

// parseRetryAfter converts a Retry-After header into a duration.
// Returns zero on parse failure (HTTP-date Retry-After is not supported
// in the SDK -- callers can inspect the raw header on the response).
func parseRetryAfter(s string) time.Duration {
	if s == "" {
		return 0
	}
	if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
