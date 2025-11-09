package signozspanmetricsconnector

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/lightstep/go-expohisto/structure"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
)

// durationToMillis converts the given duration to the number of milliseconds it represents.
// Note that this can return sub-millisecond (i.e. < 1ms) values as well.
func durationToMillis(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / float64(time.Millisecond.Nanoseconds())
}

func mapDurationsToMillis(vs []time.Duration) []float64 {
	vsm := make([]float64, len(vs))
	for i, v := range vs {
		vsm[i] = durationToMillis(v)
	}
	return vsm
}

// parseTimesFromKeyOrNow parses the time bucket prefix from a metric key and returns the StartTimeUnixNano and TimeUnixNano fields for metrics.
// Ref: https://opentelemetry.io/docs/specs/otel/metrics/data-model/#temporality .
// If the key doesn't have a time bucket prefix (in case of cumulative temporality), it uses the processor start time as StartTimeUnixNano and current time as TimeUnixNano.
// In case of delta temporality, it uses the start of the bucket as both StartTimeUnixNano & TimeUnixNano.
func parseTimesFromKeyOrNow(k metricKey, now time.Time, processorStartTime pcommon.Timestamp) (start, end pcommon.Timestamp) {
	s := string(k)
	idx := strings.IndexByte(s, 0) // first separator
	if idx <= 0 {
		return processorStartTime, pcommon.NewTimestampFromTime(now)
	}
	u, err := strconv.ParseInt(s[:idx], 10, 64)
	if err != nil {
		// cumulative temporality: StartTimeUnixNano is processor start time, TimeUnixNano is current time
		return processorStartTime, pcommon.NewTimestampFromTime(now)
	}
	bucketStart := time.Unix(u, 0)
	// delta temporality: start of the bucket is both StartTimeUnixNano & TimeUnixNano
	return pcommon.NewTimestampFromTime(bucketStart), pcommon.NewTimestampFromTime(bucketStart)
}

func newDimensions(cfgDims []Dimension) []dimension {
	if len(cfgDims) == 0 {
		return nil
	}
	dims := make([]dimension, len(cfgDims))
	for i := range cfgDims {
		dims[i].name = cfgDims[i].Name
		if cfgDims[i].Default != nil {
			val := pcommon.NewValueStr(*cfgDims[i].Default)
			dims[i].value = &val
		}
	}
	return dims
}

func getRemoteAddress(span ptrace.Span) (string, bool) {
	var addr string

	getPeerAddress := func(attrs pcommon.Map) (string, bool) {
		var addr string
		// Since net.peer.name is readable, it is preferred over net.peer.ip.
		peerName, ok := attrs.Get(conventions.AttributeNetPeerName)
		if ok {
			addr = peerName.Str()
			port, ok := attrs.Get(conventions.AttributeNetPeerPort)
			if ok {
				addr += ":" + port.Str()
			}
			return addr, true
		}
		// net.peer.name|net.host.name is renamed to server.address
		peerAddress, ok := attrs.Get("server.address")
		if ok {
			addr = peerAddress.Str()
			port, ok := attrs.Get("server.port")
			if ok {
				addr += ":" + port.Str()
			}
			return addr, true
		}

		peerIp, ok := attrs.Get(conventions.AttributeNetPeerIP)
		if ok {
			addr = peerIp.Str()
			port, ok := attrs.Get(conventions.AttributeNetPeerPort)
			if ok {
				addr += ":" + port.Str()
			}
			return addr, true
		}
		// net.peer.ip is renamed to net.sock.peer.addr
		peerAddress, ok = attrs.Get("net.sock.peer.addr")
		if ok {
			addr = peerAddress.Str()
			port, ok := attrs.Get("net.sock.peer.port")
			if ok {
				addr += ":" + port.Str()
			}
			return addr, true
		}

		// And later net.sock.peer.addr is renamed to network.peer.address
		peerAddress, ok = attrs.Get("network.peer.address")
		if ok {
			addr = peerAddress.Str()
			port, ok := attrs.Get("network.peer.port")
			if ok {
				addr += ":" + port.Str()
			}
			return addr, true
		}

		return "", false
	}

	attrs := span.Attributes()
	_, isRPC := attrs.Get(conventions.AttributeRPCSystem)
	// If the span is an RPC, the remote address is service/method.
	if isRPC {
		service, svcOK := attrs.Get(conventions.AttributeRPCService)
		if svcOK {
			addr = service.Str()
		}
		method, methodOK := attrs.Get(conventions.AttributeRPCMethod)
		if methodOK {
			addr += "/" + method.Str()
		}
		if addr != "" {
			return addr, true
		}
		// Ideally shouldn't reach here but if for some reason
		// service/method not set for RPC, fallback to peer address.
		return getPeerAddress(attrs)
	}

	// If HTTP host is set, use it.
	host, ok := attrs.Get(conventions.AttributeHTTPHost)
	if ok {
		return host.Str(), true
	}

	peerAddress, ok := getPeerAddress(attrs)
	if ok {
		// If the peer address is set and the transport is not unix domain socket, or pipe
		transport, ok := attrs.Get(conventions.AttributeNetTransport)
		if ok && transport.Str() == "unix" && transport.Str() == "pipe" {
			return "", false
		}
		return peerAddress, true
	}

	// If none of the above is set, check for full URL.
	httpURL, ok := attrs.Get(conventions.AttributeHTTPURL)
	if !ok {
		// http.url is renamed to url.full
		httpURL, ok = attrs.Get("url.full")
	}
	if ok {
		urlValue := httpURL.Str()
		// url pattern from godoc [scheme:][//[userinfo@]host][/]path[?query][#fragment]
		if !strings.HasPrefix(urlValue, "http://") && !strings.HasPrefix(urlValue, "https://") {
			urlValue = "http://" + urlValue
		}
		parsedURL, err := url.Parse(urlValue)
		if err != nil {
			return "", false
		}
		return parsedURL.Host, true
	}

	peerService, ok := attrs.Get(conventions.AttributePeerService)
	if ok {
		return peerService.Str(), true
	}

	return "", false
}

// validateDimensions checks duplicates for reserved dimensions and additional dimensions. Considering
// the usage of Prometheus related exporters, we also validate the dimensions after sanitization.
func validateDimensions(dimensions []Dimension, skipSanitizeLabel bool) error {
	labelNames := make(map[string]struct{})
	for _, key := range []string{serviceNameKey, spanKindKey, statusCodeKey} {
		labelNames[key] = struct{}{}
		labelNames[sanitize(key, skipSanitizeLabel)] = struct{}{}
	}
	labelNames[operationKey] = struct{}{}

	for _, key := range dimensions {
		if _, ok := labelNames[key.Name]; ok {
			return fmt.Errorf("duplicate dimension name %s", key.Name)
		}
		labelNames[key.Name] = struct{}{}

		sanitizedName := sanitize(key.Name, skipSanitizeLabel)
		if sanitizedName == key.Name {
			continue
		}
		if _, ok := labelNames[sanitizedName]; ok {
			return fmt.Errorf("duplicate dimension name %s after sanitization", sanitizedName)
		}
		labelNames[sanitizedName] = struct{}{}
	}

	return nil
}

// copied from prometheus-go-metric-exporter
// sanitize replaces non-alphanumeric characters with underscores in s.
func sanitize(s string, skipSanitizeLabel bool) string {
	if len(s) == 0 {
		return s
	}

	// Note: No length limit for label keys because Prometheus doesn't
	// define a length limit, thus we should NOT be truncating label keys.
	// See https://github.com/orijtech/prometheus-go-metrics-exporter/issues/4.
	s = strings.Map(sanitizeRune, s)
	if unicode.IsDigit(rune(s[0])) {
		s = "key_" + s
	}
	// replace labels starting with _ only when skipSanitizeLabel is disabled
	if !skipSanitizeLabel && strings.HasPrefix(s, "_") {
		s = "key" + s
	}
	// labels starting with __ are reserved in prometheus
	if strings.HasPrefix(s, "__") {
		s = "key" + s
	}
	return s
}

// copied from prometheus-go-metric-exporter
// sanitizeRune converts anything that is not a letter or digit to an underscore
func sanitizeRune(r rune) rune {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return r
	}
	// Everything else turns into an underscore
	return '_'
}

// getDimensionValue gets the dimension value for the given configured dimension.
// It searches through the span's attributes first, being the more specific;
// falling back to searching in resource attributes if it can't be found in the span.
// Finally, falls back to the configured default value if provided.
//
// The ok flag indicates if a dimension value was fetched in order to differentiate
// an empty string value from a state where no value was found.
func getDimensionValue(d dimension, spanAttr pcommon.Map, resourceAttr pcommon.Map) (v pcommon.Value, ok bool) {
	// The more specific span attribute should take precedence.
	if attr, exists := spanAttr.Get(d.name); exists {
		return attr, true
	} else if d.name == tagHTTPStatusCode {
		if attr, exists := spanAttr.Get(tagHTTPStatusCodeStable); exists {
			return attr, true
		}
	}
	if attr, exists := resourceAttr.Get(d.name); exists {
		return attr, true
	}
	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.value != nil {
		return *d.value, true
	}
	return v, ok
}

// TODO(srikanthccv): please check this function difference to the legacy processor implementation
func getDimensionValueWithResource(d dimension, spanAttr pcommon.Map, resourceAttr pcommon.Map) (v pcommon.Value, ok bool, foundInResource bool) {
	if attr, exists := resourceAttr.Get(d.name); exists {
		return attr, true, true
	}
	if attr, exists := spanAttr.Get(d.name); exists {
		return attr, true, false
	}
	if d.name == tagHTTPStatusCode {
		if attr, exists := spanAttr.Get(tagHTTPStatusCodeStable); exists {
			return attr, true, false
		}
	}
	// Set the default if configured, otherwise this metric will have no value set for the dimension.
	if d.value != nil {
		return *d.value, true, false
	}
	return v, ok, foundInResource
}

// setExemplars sets the histogram exemplars.
func setExemplars(exemplarsData []exemplarData, timestamp pcommon.Timestamp, exemplars pmetric.ExemplarSlice) {
	es := pmetric.NewExemplarSlice()
	es.EnsureCapacity(len(exemplarsData))

	for _, ed := range exemplarsData {
		value := ed.value
		traceID := ed.traceID
		spanID := ed.spanID

		exemplar := es.AppendEmpty()

		if traceID.IsEmpty() {
			continue
		}

		exemplar.SetDoubleValue(value)
		exemplar.SetTimestamp(timestamp)
		exemplar.SetTraceID(traceID)
		exemplar.SetSpanID(spanID)
	}

	es.CopyTo(exemplars)
}

func expoHistToExponentialDataPoint(agg *structure.Histogram[float64], dp pmetric.ExponentialHistogramDataPoint) {
	dp.SetCount(agg.Count())
	dp.SetSum(agg.Sum())
	if agg.Count() != 0 {
		dp.SetMin(agg.Min())
		dp.SetMax(agg.Max())
	}

	dp.SetZeroCount(agg.ZeroCount())
	dp.SetScale(agg.Scale())

	for _, half := range []struct {
		inFunc  func() *structure.Buckets
		outFunc func() pmetric.ExponentialHistogramDataPointBuckets
	}{
		{agg.Positive, dp.Positive},
		{agg.Negative, dp.Negative},
	} {
		in := half.inFunc()
		out := half.outFunc()
		out.SetOffset(in.Offset())
		out.BucketCounts().EnsureCapacity(int(in.Len()))

		for i := uint32(0); i < in.Len(); i++ {
			out.BucketCounts().Append(in.At(i))
		}
	}
}

func concatDimensionValue(dest *bytes.Buffer, value string, prefixSep bool) {
	if prefixSep {
		dest.WriteString(metricKeySeparator)
	}
	dest.WriteString(value)
}
