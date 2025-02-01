// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozawsfirehosereceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver"

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/SigNoz/signoz-otel-collector/config/configrouter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

const (
	headerFirehoseRequestID        = "X-Amz-Firehose-Request-Id"
	headerFirehoseCommonAttributes = "X-Amz-Firehose-Common-Attributes"
	headerContentType              = "Content-Type"
	headerContentLength            = "Content-Length"
)

var (
	errMissingHost              = errors.New("nil host")
	errInHeaderMissingRequestID = errors.New("missing request id in header")
	errInBodyMissingRequestID   = errors.New("missing request id in body")
	errInBodyDiffRequestID      = errors.New("different request id in body")
)

// The firehoseConsumer is responsible for using the unmarshaler and the consumer.
type firehoseConsumer interface {
	// Consume unmarshalls and consumes the records.
	Consume(ctx context.Context, records [][]byte, commonAttributes map[string]string) error
}

// firehoseReceiver
type firehoseReceiver struct {
	// settings is the base receiver settings.
	settings receiver.Settings
	// config is the configuration for the receiver.
	config *Config
	// server is the HTTP/HTTPS server set up to listen
	// for requests.
	server *http.Server
	// shutdownWG is the WaitGroup that is used to wait until
	// the server shutdown has completed.
	shutdownWG sync.WaitGroup
	// consumer is the firehoseConsumer to use to process/send
	// the records in each request.
	consumer firehoseConsumer
	// for testing
	address string
}

// The firehoseRequest is the format of the received request body.
type firehoseRequest struct {
	// RequestID is a GUID that should be the same value as
	// the one in the header.
	RequestID string `json:"requestId"`
	// Timestamp is the milliseconds since epoch for when the
	// request was generated.
	Timestamp int64 `json:"timestamp"`
	// Records contains the data.
	Records []firehoseRecord `json:"records"`
}

// The firehoseRecord is an individual record within the firehoseRequest.
type firehoseRecord struct {
	// Data is a base64 encoded string. Can be empty.
	Data string `json:"data"`
}

// The firehoseResponse is the expected body for the response back to
// the delivery stream.
type firehoseResponse struct {
	// RequestID is the same GUID that was received in
	// the request.
	RequestID string `json:"requestId"`
	// Timestamp is the milliseconds since epoch for when the
	// request finished being processed.
	Timestamp int64 `json:"timestamp"`
	// ErrorMessage is the error to report. Empty if request
	// was successfully processed.
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// The firehoseCommonAttributes is the format for the common attributes
// found in the header of requests.
type firehoseCommonAttributes struct {
	// CommonAttributes can be set when creating the delivery stream.
	// These will be passed to the firehoseConsumer, which should
	// attach the attributes.
	CommonAttributes map[string]string `json:"commonAttributes"`
}

var (
	_ receiver.Metrics = (*firehoseReceiver)(nil)
	_ http.Handler     = (*firehoseReceiver)(nil)
)

// Start spins up the receiver's HTTP server and makes the receiver start
// its processing.
func (fmr *firehoseReceiver) Start(ctx context.Context, host component.Host) error {
	if host == nil {
		return errMissingHost
	}

	var err error

	router := configrouter.NewDefaultMuxConfig().ToMuxRouter(fmr.settings.Logger)
	router.HandleFunc("/awsfirehose/"+fmr.config.RecordType, fmr.ServeHTTP)

	fmr.server, err = fmr.config.ServerConfig.ToServer(ctx, host, fmr.settings.TelemetrySettings, router)
	if err != nil {
		return err
	}

	var listener net.Listener
	listener, err = fmr.config.ServerConfig.ToListener(ctx)
	if err != nil {
		return err
	}
	fmr.address = listener.Addr().String()

	fmr.shutdownWG.Add(1)
	go func() {
		defer fmr.shutdownWG.Done()

		if errHTTP := fmr.server.Serve(listener); errHTTP != nil && !errors.Is(errHTTP, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(errHTTP))
		}
	}()

	return nil
}

// Shutdown tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up and
// shutting down its HTTP server.
func (fmr *firehoseReceiver) Shutdown(context.Context) error {
	if fmr.server == nil {
		return nil
	}
	err := fmr.server.Close()
	fmr.shutdownWG.Wait()
	return err
}

// ServeHTTP receives Firehose requests, unmarshalls them, and sends them along to the firehoseConsumer,
// which is responsible for unmarshalling the records and sending them to the next consumer.
func (fmr *firehoseReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	requestID := r.Header.Get(headerFirehoseRequestID)
	if requestID == "" {
		fmr.settings.Logger.Error("Invalid Firehose request", zap.Error(errInHeaderMissingRequestID))
		fmr.sendResponse(w, requestID, http.StatusBadRequest, configrouter.FromError(errInHeaderMissingRequestID, codes.InvalidArgument))
		return
	}

	body, err := fmr.getBody(r)
	if err != nil {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, configrouter.FromError(err, codes.InvalidArgument))
		return
	}

	var fr firehoseRequest
	if err = json.Unmarshal(body, &fr); err != nil {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, configrouter.FromError(err, codes.InvalidArgument))
		return
	}

	if fr.RequestID == "" {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, configrouter.FromError(errInBodyMissingRequestID, codes.InvalidArgument))
		return
	} else if fr.RequestID != requestID {
		fmr.sendResponse(w, requestID, http.StatusBadRequest, configrouter.FromError(errInBodyDiffRequestID, codes.InvalidArgument))
		return
	}

	records := make([][]byte, 0, len(fr.Records))
	for index, record := range fr.Records {
		if record.Data != "" {
			var decoded []byte
			decoded, err = base64.StdEncoding.DecodeString(record.Data)
			if err != nil {
				fmr.sendResponse(w, requestID, http.StatusBadRequest, configrouter.FromError(fmt.Errorf("unable to base64 decode the record at index %d: %w", index, err), codes.InvalidArgument))
				return
			}
			records = append(records, decoded)
		}
	}

	commonAttributes, err := fmr.getCommonAttributes(r)
	if err != nil {
		fmr.settings.Logger.Error("Unable to get common attributes from request header. Will not attach attributes.", zap.Error(err))
	}

	err = fmr.consumer.Consume(ctx, records, commonAttributes)
	if err != nil {
		fmr.settings.Logger.Error("Unable to consume records", zap.Error(err))
		fmr.sendResponse(w, requestID, http.StatusInternalServerError, err)
		return
	}

	fmr.sendResponse(w, requestID, http.StatusOK, nil)
}

// getBody reads the body from the request as a slice of bytes.
func (fmr *firehoseReceiver) getBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	err = r.Body.Close()
	if err != nil {
		return nil, err
	}
	return body, nil
}

// getCommonAttributes unmarshalls the common attributes from the request header
func (fmr *firehoseReceiver) getCommonAttributes(r *http.Request) (map[string]string, error) {
	attributes := make(map[string]string)
	if commonAttributes := r.Header.Get(headerFirehoseCommonAttributes); commonAttributes != "" {
		var fca firehoseCommonAttributes
		if err := json.Unmarshal([]byte(commonAttributes), &fca); err != nil {
			return nil, err
		}
		attributes = fca.CommonAttributes
	}
	return attributes, nil
}

// sendResponse writes a response to Firehose in the expected format.
func (fmr *firehoseReceiver) sendResponse(w http.ResponseWriter, requestID string, statusCode int, err error) {
	var errorMessage string
	if err != nil {
		errorMessage = err.Error()
	}
	body := firehoseResponse{
		RequestID:    requestID,
		Timestamp:    time.Now().UnixMilli(),
		ErrorMessage: errorMessage,
	}

	if err != nil {
		configrouter.WriteErrorb(w, err, body)
		return
	}

	configrouter.WriteSuccessb(w, body, statusCode)
}
