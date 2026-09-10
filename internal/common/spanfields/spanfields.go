// Package spanfields derives the span fields that SigNoz stores beside the raw
// span attributes: the calculated HTTP, database and status fields and the
// remote flag. The traces exporter and the metadata exporter share it so both
// write the same values.
package spanfields

import (
	"net/url"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	hasIsRemoteMask uint32 = 0x00000100
	isRemoteMask    uint32 = 0x00000200
)

// Calculated holds the span fields derived from the span attributes.
type Calculated struct {
	HttpMethod         string
	HttpHost           string
	HttpUrl            string
	ResponseStatusCode string
	DBName             string
	DBOperation        string
	ExternalHttpMethod string
	ExternalHttpUrl    string
}

var hostAttributes = map[string]struct{}{
	"http.host":                {},
	"server.address":           {},
	"client.address":           {},
	"http.request.header.host": {},
	"net.peer.name":            {},
}

// CalculatedFrom derives the calculated fields from the span attributes. The
// external fields are set for client spans only; the host is taken from a host
// attribute when present and from the URL of a client span otherwise.
func CalculatedFrom(attributes pcommon.Map, kind ptrace.SpanKind) Calculated {
	var c Calculated
	client := kind == ptrace.SpanKindClient
	attributes.Range(func(k string, v pcommon.Value) bool {
		switch {
		case k == "http.status_code" || k == "http.response.status_code":
			c.ResponseStatusCode = statusCode(v)
		case (k == "http.url" || k == "url.full") && client:
			value := v.Str()
			if parsed, err := url.Parse(value); err == nil {
				value = parsed.Hostname()
			}
			c.ExternalHttpUrl = value
			c.HttpUrl = v.Str()
			if c.HttpHost == "" {
				c.HttpHost = value
			}
		case (k == "http.method" || k == "http.request.method") && client:
			c.ExternalHttpMethod = v.Str()
			c.HttpMethod = v.Str()
		case k == "http.url" || k == "url.full":
			c.HttpUrl = v.Str()
		case k == "http.method" || k == "http.request.method":
			c.HttpMethod = v.Str()
		case k == "db.name" || k == "db.namespace":
			c.DBName = v.Str()
		case k == "db.operation" || k == "db.operation.name":
			c.DBOperation = v.Str()
		case k == "rpc.grpc.status_code":
			c.ResponseStatusCode = statusCode(v)
		case k == "rpc.jsonrpc.error_code":
			c.ResponseStatusCode = v.Str()
		default:
			if _, ok := hostAttributes[k]; ok {
				c.HttpHost = v.Str()
			}
		}
		return true
	})
	return c
}

// statusCode formats a status code that arrives as either a string or an
// integer as its decimal form.
func statusCode(v pcommon.Value) string {
	statusInt := v.Int()
	if parsed, err := strconv.Atoi(v.Str()); err == nil && parsed != 0 {
		statusInt = int64(parsed)
	}
	return strconv.FormatInt(statusInt, 10)
}

// IsRemote reports the span's remote flag as "yes", "no" or "unknown" when
// the flags do not carry it.
func IsRemote(flags uint32) string {
	if flags&hasIsRemoteMask == 0 {
		return "unknown"
	}
	if flags&isRemoteMask != 0 {
		return "yes"
	}
	return "no"
}
