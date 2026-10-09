package fieldvaluestest

import (
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Cluster is a Kubernetes cluster whose pods change from day to day, for the
// churn load test. Each day a share of the deployments roll: their pods get
// new names, uids and container ids, as after a deploy.
type Cluster struct {
	rnd         *rand.Rand
	deployments []deployment
	pods        []pod
	nextID      int
}

type deployment struct {
	name      string
	namespace string
	revision  int
	routes    []string
}

type pod struct {
	deployment int
	name       string
	uid        string
	container  string
	node       int
}

const (
	replicas        = 3
	routesPerDeploy = 10
	nodes           = 40
)

// ResourceAttributes is the number of resource attributes of each pod.
const ResourceAttributes = 21

// InfraMetrics and AppMetrics are the metrics of each pod. Infra metrics keep
// the identity in resource attributes and have 0 to 2 point labels; app
// metrics have route, method and status labels.
var (
	InfraMetrics = infraMetrics()
	AppMetrics   = []string{
		"http.server.request.duration", "http.server.request.body.size", "http.server.response.body.size", "rpc.server.duration",
		"app.requests", "app.errors", "app.queue.depth", "app.cache.hits", "app.cache.misses", "app.jobs",
	}
)

type infraMetric struct {
	name   string
	labels [][2]string
}

func infraMetrics() []infraMetric {
	var out []infraMetric
	for i := 0; i < 30; i++ {
		out = append(out, infraMetric{name: fmt.Sprintf("k8s.pod.metric_%02d", i)})
	}
	for i := 0; i < 6; i++ {
		out = append(out, infraMetric{name: fmt.Sprintf("k8s.pod.directional_%02d", i), labels: [][2]string{{"direction", "receive"}, {"direction", "transmit"}}})
	}
	for i := 0; i < 4; i++ {
		out = append(out, infraMetric{name: fmt.Sprintf("k8s.pod.network_%02d", i), labels: [][2]string{{"interface", "eth0"}, {"direction", "receive"}}})
	}
	return out
}

// NewCluster makes a cluster of about pods pods.
func NewCluster(seed int64, pods int) *Cluster {
	c := &Cluster{rnd: rand.New(rand.NewSource(seed))}
	for d := 0; d < max(pods/replicas, 1); d++ {
		dep := deployment{name: fmt.Sprintf("deploy-%03d", d), namespace: fmt.Sprintf("ns-%02d", d%10)}
		for r := 0; r < routesPerDeploy; r++ {
			dep.routes = append(dep.routes, fmt.Sprintf("/%s/route-%02d", dep.name, r))
		}
		c.deployments = append(c.deployments, dep)
		for r := 0; r < replicas; r++ {
			c.pods = append(c.pods, c.newPod(d))
		}
	}
	return c
}

func (c *Cluster) newPod(d int) pod {
	c.nextID++
	dep := c.deployments[d]
	return pod{
		deployment: d,
		name:       fmt.Sprintf("%s-%d-%05x", dep.name, dep.revision, c.nextID),
		uid:        fmt.Sprintf("uid-%08x", c.nextID),
		container:  fmt.Sprintf("%016x", c.rnd.Uint64()),
		node:       c.rnd.Intn(nodes),
	}
}

// Roll replaces the pods of share of the deployments and returns the number
// of new pods.
func (c *Cluster) Roll(share float64) int {
	rolled := 0
	for d := range c.deployments {
		if c.rnd.Float64() >= share {
			continue
		}
		c.deployments[d].revision++
		for i := range c.pods {
			if c.pods[i].deployment == d {
				c.pods[i] = c.newPod(d)
				rolled++
			}
		}
	}
	return rolled
}

// Pods is the number of pods now.
func (c *Cluster) Pods() int {
	return len(c.pods)
}

func (c *Cluster) resource(p pod, attrs pcommon.Map) {
	dep := c.deployments[p.deployment]
	node := fmt.Sprintf("node-%02d", p.node)
	attrs.PutStr("k8s.cluster.name", "prod-eu")
	attrs.PutStr("k8s.namespace.name", dep.namespace)
	attrs.PutStr("k8s.deployment.name", dep.name)
	attrs.PutStr("k8s.replicaset.name", fmt.Sprintf("%s-%d", dep.name, dep.revision))
	attrs.PutStr("k8s.pod.name", p.name)
	attrs.PutStr("k8s.pod.uid", p.uid)
	attrs.PutStr("k8s.node.name", node)
	attrs.PutStr("k8s.container.name", "app")
	attrs.PutStr("container.id", p.container)
	attrs.PutStr("host.name", node)
	attrs.PutStr("service.name", dep.name)
	attrs.PutStr("service.version", fmt.Sprintf("v%d", dep.revision))
	attrs.PutStr("service.namespace", dep.namespace)
	attrs.PutStr("deployment.environment.name", "prod")
	attrs.PutStr("cloud.provider", "aws")
	attrs.PutStr("cloud.region", "eu-west-1")
	attrs.PutStr("cloud.availability_zone", fmt.Sprintf("eu-west-1%c", 'a'+p.node%3))
	attrs.PutStr("os.type", "linux")
	attrs.PutStr("telemetry.sdk.name", "opentelemetry")
	attrs.PutStr("telemetry.sdk.language", "go")
	attrs.PutStr("telemetry.sdk.version", "1.30.0")
}

// Logs makes the logs of one day: records per pod, in batches of about
// batch records.
func (c *Cluster) Logs(day time.Time, perPod, batch int) []plog.Logs {
	var out []plog.Logs
	ld := plog.NewLogs()
	n := 0
	for _, p := range c.pods {
		rl := ld.ResourceLogs().AppendEmpty()
		c.resource(p, rl.Resource().Attributes())
		sl := rl.ScopeLogs().AppendEmpty()
		sl.Scope().SetName("app-logger")
		routes := c.deployments[p.deployment].routes
		for i := 0; i < perPod; i++ {
			route := c.rnd.Intn(len(routes))
			lr := sl.LogRecords().AppendEmpty()
			lr.SetTimestamp(pcommon.NewTimestampFromTime(day.Add(time.Duration(c.rnd.Intn(3600)) * time.Second)))
			severity, status := "INFO", int64(200)
			if c.rnd.Intn(10) == 0 {
				severity, status = "ERROR", 500
			}
			lr.SetSeverityText(severity)
			lr.Attributes().PutStr("http.route", routes[route])
			lr.Attributes().PutStr("http.request.method", methods[route%len(methods)])
			lr.Attributes().PutInt("http.response.status_code", status)
			lr.Attributes().PutStr("request.id", fmt.Sprintf("%016x", c.rnd.Uint64()))
		}
		n += perPod
		if n >= batch {
			out = append(out, ld)
			ld, n = plog.NewLogs(), 0
		}
	}
	if n > 0 {
		out = append(out, ld)
	}
	return out
}

// Metrics makes one point per series per pod for one day, in batches of
// about batch points.
func (c *Cluster) Metrics(day time.Time, batch int) []pmetric.Metrics {
	var out []pmetric.Metrics
	md := pmetric.NewMetrics()
	n := 0
	ts := pcommon.NewTimestampFromTime(day)
	for _, p := range c.pods {
		rm := md.ResourceMetrics().AppendEmpty()
		c.resource(p, rm.Resource().Attributes())
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName("kubeletstats")
		for _, im := range InfraMetrics {
			m := sm.Metrics().AppendEmpty()
			m.SetName(im.name)
			g := m.SetEmptyGauge()
			if len(im.labels) == 0 {
				g.DataPoints().AppendEmpty().SetTimestamp(ts)
				n++
				continue
			}
			for _, l := range im.labels {
				dp := g.DataPoints().AppendEmpty()
				dp.SetTimestamp(ts)
				dp.Attributes().PutStr(l[0], l[1])
				if l[0] == "interface" {
					dp.Attributes().PutStr("direction", "transmit")
				}
				n++
			}
		}
		routes := c.deployments[p.deployment].routes
		for i, name := range AppMetrics {
			m := sm.Metrics().AppendEmpty()
			m.SetName(name)
			var points pmetric.NumberDataPointSlice
			var histogram pmetric.HistogramDataPointSlice
			if i < 4 {
				h := m.SetEmptyHistogram()
				h.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				histogram = h.DataPoints()
			} else {
				s := m.SetEmptySum()
				s.SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)
				points = s.DataPoints()
			}
			for r, route := range routes {
				for _, status := range []string{"200", "500"} {
					if status == "500" && r%2 == 1 {
						continue
					}
					var attrs pcommon.Map
					if i < 4 {
						dp := histogram.AppendEmpty()
						dp.SetTimestamp(ts)
						attrs = dp.Attributes()
					} else {
						dp := points.AppendEmpty()
						dp.SetTimestamp(ts)
						attrs = dp.Attributes()
					}
					attrs.PutStr("http.route", route)
					attrs.PutStr("http.request.method", methods[r%len(methods)])
					attrs.PutStr("http.response.status_code", status)
					n++
				}
			}
		}
		if n >= batch {
			out = append(out, md)
			md, n = pmetric.NewMetrics(), 0
		}
	}
	if n > 0 {
		out = append(out, md)
	}
	return out
}
