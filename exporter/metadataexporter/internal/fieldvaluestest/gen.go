// Package fieldvaluestest generates telemetry with a known shape for the
// benchmarks and load tests of the field values store.
//
// Each signal has 200 resources: 50 services with 4 pods each. Record fields
// have few values (method, route, status, region, severity) and two have one
// value per record (user.id, request.id), so that the store meets dimension
// fields, identifiers, and resources that reach the set limit.
package fieldvaluestest

import (
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	Services          = 50
	PodsPerService    = 4
	RoutesPerService  = 40
	ResourcesPerBatch = 20
)

var (
	methods    = []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	statuses   = []int64{200, 201, 204, 301, 400, 404, 500, 503}
	regions    = []string{"us-east-1", "us-west-2", "eu-west-1", "eu-central-1", "ap-south-1", "ap-northeast-1"}
	severities = []string{"DEBUG", "INFO", "WARN", "ERROR"}
	exceptions = []string{"TimeoutError", "ConnectionRefused", "NullPointer", "IndexOutOfRange", "Unauthorized"}
)

// Generator makes batches. It is not safe for concurrent use.
type Generator struct {
	rnd *rand.Rand
	seq int64
	now time.Time
	// Correlated makes the fields depend on each other, as in real data: the
	// route fixes the method, the pod fixes the region, and most requests
	// succeed with severity INFO. Without it, every field is independent, the
	// worst case for the number of sets.
	Correlated bool
	pod        int
}

func NewGenerator(seed int64, now time.Time) *Generator {
	return &Generator{rnd: rand.New(rand.NewSource(seed)), now: now}
}

func (g *Generator) resource(attrs pcommon.Map) (service, pod int) {
	service = g.rnd.Intn(Services)
	pod = g.rnd.Intn(PodsPerService)
	g.pod = service*PodsPerService + pod
	attrs.PutStr("service.name", fmt.Sprintf("svc-%02d", service))
	attrs.PutStr("k8s.namespace.name", fmt.Sprintf("ns-%d", service%5))
	attrs.PutStr("k8s.pod.name", fmt.Sprintf("svc-%02d-pod-%d", service, pod))
	attrs.PutStr("host.name", fmt.Sprintf("node-%02d", (service*PodsPerService+pod)%20))
	attrs.PutStr("deployment.environment.name", "prod")
	return service, pod
}

func (g *Generator) ts() pcommon.Timestamp {
	return pcommon.NewTimestampFromTime(g.now.Add(-time.Duration(g.rnd.Intn(3600)) * time.Second))
}

func (g *Generator) recordAttrs(attrs pcommon.Map, service, route int) {
	g.seq++
	method := methods[g.rnd.Intn(len(methods))]
	status := statuses[g.rnd.Intn(len(statuses))]
	region := regions[g.rnd.Intn(len(regions))]
	if g.Correlated {
		method = methods[route%len(methods)]
		if g.rnd.Intn(10) < 9 {
			status = 200
		}
		region = regions[g.pod%len(regions)]
	}
	attrs.PutStr("http.method", method)
	attrs.PutStr("http.route", fmt.Sprintf("/svc-%02d/route-%02d", service, route))
	attrs.PutInt("http.status_code", status)
	attrs.PutStr("cloud.region", region)
	attrs.PutStr("user.id", fmt.Sprintf("user-%d", g.seq))
	attrs.PutStr("request.id", fmt.Sprintf("%016x", g.rnd.Uint64()))
}

func (g *Generator) severity() string {
	if g.Correlated && g.rnd.Intn(10) < 8 {
		return "INFO"
	}
	return severities[g.rnd.Intn(len(severities))]
}

// Logs makes a batch of n log records over ResourcesPerBatch resources.
func (g *Generator) Logs(n int) plog.Logs {
	ld := plog.NewLogs()
	perResource := max(n/ResourcesPerBatch, 1)
	for made := 0; made < n; {
		rl := ld.ResourceLogs().AppendEmpty()
		service, _ := g.resource(rl.Resource().Attributes())
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("app-logger")
		for i := 0; i < perResource && made < n; i++ {
			lr := sl.LogRecords().AppendEmpty()
			lr.SetTimestamp(g.ts())
			lr.SetSeverityText(g.severity())
			lr.Body().SetStr("request handled")
			g.recordAttrs(lr.Attributes(), service, g.rnd.Intn(RoutesPerService))
			made++
		}
	}
	return ld
}

// Traces makes a batch of n spans over ResourcesPerBatch resources, with up
// to two events per span.
func (g *Generator) Traces(n int) ptrace.Traces {
	td := ptrace.NewTraces()
	perResource := max(n/ResourcesPerBatch, 1)
	for made := 0; made < n; {
		rs := td.ResourceSpans().AppendEmpty()
		service, _ := g.resource(rs.Resource().Attributes())
		ss := rs.ScopeSpans().AppendEmpty()
		ss.Scope().SetName("otel-go")
		for i := 0; i < perResource && made < n; i++ {
			span := ss.Spans().AppendEmpty()
			start := g.ts()
			span.SetStartTimestamp(start)
			span.SetEndTimestamp(start + pcommon.Timestamp(g.rnd.Intn(1e9)))
			route := g.rnd.Intn(RoutesPerService)
			attrRoute := g.rnd.Intn(RoutesPerService)
			span.SetName(fmt.Sprintf("route-%02d", route))
			span.SetKind(ptrace.SpanKind(1 + g.rnd.Intn(3)))
			if g.Correlated {
				attrRoute = route
				span.SetKind(ptrace.SpanKind(1 + route%3))
			}
			if g.rnd.Intn(10) == 0 {
				span.Status().SetCode(ptrace.StatusCodeError)
			}
			g.recordAttrs(span.Attributes(), service, attrRoute)
			for e := g.rnd.Intn(3); e > 0; e-- {
				ev := span.Events().AppendEmpty()
				ev.SetName("exception")
				ev.Attributes().PutStr("exception.type", exceptions[g.rnd.Intn(len(exceptions))])
			}
			made++
		}
	}
	return td
}

// Metrics makes a batch of n data points of 20 sum metrics over
// ResourcesPerBatch resources. Each metric has up to 8 x 5 x 40 series per
// resource.
func (g *Generator) Metrics(n int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	perResource := max(n/ResourcesPerBatch, 1)
	for made := 0; made < n; {
		rm := md.ResourceMetrics().AppendEmpty()
		service, _ := g.resource(rm.Resource().Attributes())
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("otel-go")
		m := sm.Metrics().AppendEmpty()
		m.SetName(fmt.Sprintf("app_metric_%02d", g.rnd.Intn(20)))
		sum := m.SetEmptySum()
		sum.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
		for i := 0; i < perResource && made < n; i++ {
			dp := sum.DataPoints().AppendEmpty()
			dp.SetTimestamp(g.ts())
			dp.SetDoubleValue(g.rnd.Float64())
			dp.Attributes().PutStr("code", fmt.Sprint(statuses[g.rnd.Intn(len(statuses))]))
			dp.Attributes().PutStr("method", methods[g.rnd.Intn(len(methods))])
			dp.Attributes().PutStr("route", fmt.Sprintf("/svc-%02d/route-%02d", service, g.rnd.Intn(RoutesPerService)))
			made++
		}
	}
	return md
}
