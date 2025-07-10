package configrouter

import (
	"net/http"
	"strconv"

	"github.com/goccy/go-json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func WriteSuccessb(w http.ResponseWriter, body any, statusCode int) {
	if body == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		return
	}

	successb, err := json.Marshal(body)
	if err != nil {
		WriteError(w, FromStatus(status.New(codes.Internal, "failed to marshal body")))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(successb)))
	w.WriteHeader(statusCode)
	_, _ = w.Write(successb)
}

// Writes an error with an input body
func WriteErrorb(w http.ResponseWriter, err error, body any) {
	if body == nil {
		WriteError(w, err)
		return
	}

	berr := Unwrapb(err)
	errb, merr := json.Marshal(body)
	if merr != nil {
		WriteError(w, merr)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.Itoa(len(errb)))
	w.WriteHeader(berr.c)
	_, _ = w.Write(errb)
}

// Writes an error to the response writer.
func WriteError(w http.ResponseWriter, err error) {
	berr := Unwrapb(err)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(berr.c)
	_, _ = w.Write(berr.b)
}
