package configrouter

import (
	"net/http"

	"github.com/goccy/go-json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type base struct {
	// e is the actual error being wrapped
	e error
	// c is the actual derived http code
	c int
	// m is the error message to be displayed to the user
	m string
	// b is the body to be sent in the http response
	b []byte
}

func (b *base) Error() string {
	return b.m
}

// FromError returns an error of base representation.
//
//   - If err is of type `*status.Status`, an http status code is derived from
//     the corresponding `*status.Status`.Code. If the status code cannot be computed,
//		the input fallback status code is used. The http response body is the json
//     representation of the “*status.Status“ protobuf. This representation looks like:
//     `{"code":`*status.Status`.Code, "message":`*status.Status`.Message}`
//
//   - If err is not of type `*status.Status`, an error of type `*status.Status` is
//     created from this err. This created `*status.Status` has its code set to
//     `codes.Internal` and message set to `err.Error()`.

func FromError(e error, c codes.Code) *base {
	if s, ok := status.FromError(e); ok {
		return FromStatus(s)
	}

	return FromStatus(status.New(c, e.Error()))
}

func FromStatus(s *status.Status) *base {
	c := codeFromStatus(s, http.StatusInternalServerError)
	b := bodyFromStatus(s)

	return &base{
		e: s.Err(),
		c: c,
		b: b,
		m: s.Message(),
	}

}

func (b *base) WithCode(c int) *base {
	b.c = c
	return b
}

func Unwrapb(err error) *base {
	if b, ok := err.(*base); ok {
		return b
	}

	return FromError(err, codes.Internal)
}

func codeFromStatus(s *status.Status, code int) int {
	switch s.Code() {
	// Retryable
	case codes.Canceled, codes.DeadlineExceeded, codes.Aborted, codes.OutOfRange, codes.Unavailable, codes.DataLoss:
		return http.StatusServiceUnavailable
	// Retryable
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	// Non Retryable
	case codes.InvalidArgument:
		return http.StatusBadRequest
	// Non Retryable
	case codes.Internal:
		return http.StatusInternalServerError
	// Non Retryable
	case codes.NotFound:
		return http.StatusNotFound
	default:
		return code
	}
}

func bodyFromStatus(s *status.Status) []byte {
	body, merr := json.Marshal(s.Proto())
	if merr != nil {
		s := status.New(codes.Internal, "failed to marshal error message").Proto().String()
		return []byte(s)
	}

	return body
}
