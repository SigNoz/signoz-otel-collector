package configrouter

import (
	"encoding/json"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Writes an error to the response writer.
// If the error is a gRPC status, it will use the gRPC status code to determine the HTTP status code.
func WriteError(w http.ResponseWriter, err error, statusCode int) {
	var body []byte
	if s, ok := status.FromError(err); ok {
		body = bodyFromStatus(s)
		statusCode = httpStatusCodeFromStatus(s, statusCode)
	} else {
		body = []byte(err.Error())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(body)
}

// Returns the response body from a gRPC status.
func bodyFromStatus(s *status.Status) []byte {
	body, merr := json.Marshal(s.Proto())
	if merr != nil {
		return []byte(`{"code": 13, "message": "failed to marshal error message"}`)
	}

	return body
}

// Returns the HTTP status code from a gRPC status code.
func httpStatusCodeFromStatus(s *status.Status, statusCode int) int {
	switch s.Code() {
	// Retryable
	case codes.Canceled, codes.DeadlineExceeded, codes.Aborted, codes.OutOfRange, codes.Unavailable, codes.DataLoss:
		return http.StatusServiceUnavailable
	// Retryable
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	// Return the statusCode received
	default:
		return statusCode
	}
}
