// Package spanfields computes the span fields that SigNoz derives from span
// attributes and flags, such as http_method and response_status_code. The
// traces exporter stores them as columns, and the metadata exporter writes
// their values for suggestions, so both must compute them the same way.
package spanfields

import (
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

const (
	hasIsRemoteMask uint32 = 0x00000100
	isRemoteMask    uint32 = 0x00000200

	spanKindClient = 3
)

var possibleHostAttr = map[string]struct{}{
	"http.host":                {},
	"server.address":           {},
	"client.address":           {},
	"http.request.header.host": {},
	"net.peer.name":            {},
}

// Calculated holds the fields derived from the attributes of a span. An empty
// string means that the span has no value for the field.
type Calculated struct {
	ResponseStatusCode string
	HTTPURL            string
	HTTPHost           string
	HTTPMethod         string
	ExternalHTTPURL    string
	ExternalHTTPMethod string
	DBName             string
	DBOperation        string
}

// Calculate derives the fields from the attributes of a span of the given
// kind (the OTLP span kind number).
func Calculate(attributes pcommon.Map, kind int8) Calculated {
	var c Calculated
	attributes.Range(func(k string, v pcommon.Value) bool {
		if k == "http.status_code" || k == "http.response.status_code" {
			// Handle both string/int http status codes.
			c.ResponseStatusCode = statusCode(v)
		} else if (k == "http.url" || k == "url.full") && kind == spanKindClient {
			value := v.Str()
			valueURL, err := url.Parse(value)
			if err == nil {
				value = valueURL.Hostname()
			}
			c.ExternalHTTPURL = value
			c.HTTPURL = v.Str()
			if c.HTTPHost == "" { // skip override if already set using possibleHostAttr
				c.HTTPHost = value
			}
		} else if (k == "http.method" || k == "http.request.method") && kind == spanKindClient {
			c.ExternalHTTPMethod = v.Str()
			c.HTTPMethod = v.Str()
		} else if (k == "http.url" || k == "url.full") && kind != spanKindClient {
			c.HTTPURL = v.Str()
		} else if (k == "http.method" || k == "http.request.method") && kind != spanKindClient {
			c.HTTPMethod = v.Str()
		} else if _, ok := possibleHostAttr[k]; ok {
			c.HTTPHost = v.Str()
		} else if k == "db.name" || k == "db.namespace" {
			c.DBName = v.Str()
		} else if k == "db.operation" || k == "db.operation.name" {
			c.DBOperation = v.Str()
		} else if k == "rpc.grpc.status_code" {
			// Handle both string/int status code in GRPC spans.
			c.ResponseStatusCode = statusCode(v)
		} else if k == "rpc.jsonrpc.error_code" {
			c.ResponseStatusCode = v.Str()
		}
		return true
	})
	return c
}

// statusCode gives a status code held as an int or as a string. A string that
// is not a number gives "0", as do types other than int and string. Atoi runs
// only on strings, because its error allocates.
func statusCode(v pcommon.Value) string {
	statusInt := v.Int()
	if v.Type() == pcommon.ValueTypeStr {
		if statusString, err := strconv.Atoi(v.Str()); err == nil && statusString != 0 {
			statusInt = int64(statusString)
		}
	}
	return strconv.FormatInt(statusInt, 10)
}

// IsRemote gives "yes", "no" or "unknown" from the flags of a span.
func IsRemote(flags uint32) string {
	if flags&hasIsRemoteMask == 0 {
		return "unknown"
	}
	if flags&isRemoteMask != 0 {
		return "yes"
	}
	return "no"
}
