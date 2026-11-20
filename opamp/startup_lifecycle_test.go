// Real upstream collector, component registry, supervisor and WebSocket client.
// Only the local OpAMP server and telemetry destination are test peers.
package opamp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SigNoz/signoz-otel-collector/signozcol"
	"github.com/gorilla/websocket"
	"github.com/knadh/koanf"
	"github.com/open-telemetry/opamp-go/client"
	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.opentelemetry.io/collector/pdata/pmetric/pmetricotlp"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type qualificationPeer struct {
	mu          sync.Mutex
	config      []byte
	conns       map[*websocket.Conn][]byte
	statuses    chan *protobufs.RemoteConfigStatus
	connections atomic.Int64
	server      *httptest.Server
}

func qualificationServer(t *testing.T, config []byte) *qualificationPeer {
	p := &qualificationPeer{config: config, conns: map[*websocket.Conn][]byte{}, statuses: make(chan *protobufs.RemoteConfigStatus, 100)}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		defer func() { p.mu.Lock(); delete(p.conns, c); p.mu.Unlock() }()
		first := true
		for {
			kind, data, err := c.ReadMessage()
			if err != nil {
				return
			}
			if kind != websocket.BinaryMessage || len(data) == 0 {
				continue
			}
			if data[0] == 0 {
				data = data[1:]
			}
			msg := &protobufs.AgentToServer{}
			if err := proto.Unmarshal(data, msg); err != nil {
				t.Errorf("decode OpAMP: %v", err)
				return
			}
			if status := msg.RemoteConfigStatus; status != nil {
				select {
				case p.statuses <- status:
				default:
				}
			}
			if first {
				first = false
				p.mu.Lock()
				p.conns[c] = append([]byte(nil), msg.InstanceUid...)
				p.connections.Add(1)
				if p.config != nil {
					p.sendLocked(c, p.config)
				}
				p.mu.Unlock()
			}
		}
	}))
	t.Cleanup(func() { p.disconnect(); p.server.Close() })
	return p
}

func (p *qualificationPeer) sendLocked(c *websocket.Conn, body []byte) {
	msg := &protobufs.ServerToAgent{InstanceUid: p.conns[c], RemoteConfig: &protobufs.AgentRemoteConfig{
		ConfigHash: fileHash(body), Config: &protobufs.AgentConfigMap{ConfigMap: map[string]*protobufs.AgentConfigFile{
			"collector.yaml": {Body: body, ContentType: "text/yaml"},
		}},
	}}
	data, _ := proto.Marshal(msg)
	_ = c.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_ = c.WriteMessage(websocket.BinaryMessage, append([]byte{0}, data...))
}

func (p *qualificationPeer) publish(body []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = body
	for c := range p.conns {
		p.sendLocked(c, body)
	}
}

func (p *qualificationPeer) disconnect() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.conns {
		_ = c.Close()
	}
}

func (p *qualificationPeer) awaitStatus(t *testing.T, body []byte, status protobufs.RemoteConfigStatuses) {
	t.Helper()
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case got := <-p.statuses:
			if bytes.Equal(got.LastRemoteConfigHash, fileHash(body)) && got.Status == status {
				return
			}
		case <-deadline.C:
			t.Fatalf("no matching OpAMP status: %v", status)
		}
	}
}

// Timing instrument, not a transport double: the real WebSocket Start executes
// unchanged, then this wrapper waits for its first real callback to complete.
// This forces the early-callback schedule without relying on timing luck.
type qualificationBarrier struct{ client.OpAMPClient }

func (b qualificationBarrier) Start(ctx context.Context, settings types.StartSettings) error {
	done := make(chan struct{})
	var once sync.Once
	onMessage := settings.Callbacks.OnMessage
	settings.Callbacks.OnMessage = func(ctx context.Context, msg *types.MessageData) {
		onMessage(ctx, msg)
		if msg.RemoteConfig != nil {
			once.Do(func() { close(done) })
		}
	}
	if err := b.OpAMPClient.Start(ctx, settings); err != nil {
		return err
	}
	select {
	case <-done:
		return nil
	case <-time.After(15 * time.Second):
		return fmt.Errorf("early callback timeout")
	}
}

func qualificationAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	a := l.Addr().String()
	_ = l.Close()
	return a
}

func qualificationOpen(address string) bool {
	c, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func qualificationEventually(t *testing.T, check func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal(message)
}

type qualificationFixture struct {
	s              *serverClient
	p              *qualificationPeer
	config         []byte
	ingest, health string
	points         chan int64
	stopped        bool
}

func qualificationNew(t *testing.T, delivery string) *qualificationFixture {
	t.Helper()
	// UpsertInstanceID uses a package-level parser. Isolate test configurations
	// from preceding cases and restore it after all fixture cleanup completes.
	previousParser := k
	k = koanf.New("::")
	t.Cleanup(func() { k = previousParser })
	f := &qualificationFixture{ingest: qualificationAddress(t), health: qualificationAddress(t), points: make(chan int64, 100)}
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			t.Errorf("read telemetry: %v", err)
			w.WriteHeader(500)
			return
		}
		req := pmetricotlp.NewExportRequest()
		if err := req.UnmarshalProto(body); err != nil {
			t.Errorf("decode telemetry: %v", err)
			w.WriteHeader(400)
			return
		}
		for i := 0; i < req.Metrics().ResourceMetrics().Len(); i++ {
			scopes := req.Metrics().ResourceMetrics().At(i).ScopeMetrics()
			for j := 0; j < scopes.Len(); j++ {
				metrics := scopes.At(j).Metrics()
				for k := 0; k < metrics.Len(); k++ {
					m := metrics.At(k)
					if m.Name() == "opamp_qualification_marker" && m.Gauge().DataPoints().Len() == 1 {
						f.points <- m.Gauge().DataPoints().At(0).IntValue()
					}
				}
			}
		}
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(200)
	}))
	t.Cleanup(sink.Close)
	f.config = []byte(fmt.Sprintf(`receivers:
  otlp:
    protocols:
      http:
        endpoint: %s
exporters:
  otlphttp:
    endpoint: %s
    compression: none
    retry_on_failure:
      enabled: false
    sending_queue:
      enabled: false
extensions:
  health_check:
    endpoint: %s
service:
  telemetry:
    logs:
      level: error
    metrics:
      level: none
  extensions: [health_check]
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [otlphttp]
`, f.ingest, sink.URL, f.health))
	var initial []byte
	if delivery != "delayed" {
		initial = f.config
	}
	f.p = qualificationServer(t, initial)
	path := filepath.Join(t.TempDir(), "collector.yaml")
	if err := os.WriteFile(path, f.config, 0600); err != nil {
		t.Fatal(err)
	}
	coll := signozcol.New(signozcol.WrappedCollectorSettings{ConfigPaths: []string{path}, Version: "opamp-local-qualification", PollInterval: 50 * time.Millisecond})
	instance, err := NewServerClient(&NewServerClientOpts{Logger: zap.NewNop(), Config: &AgentManagerConfig{ServerEndpoint: "ws" + strings.TrimPrefix(f.p.server.URL, "http")}, WrappedCollector: coll, CollectorConfigPath: path})
	if err != nil {
		t.Fatal(err)
	}
	f.s = instance.(*serverClient)
	if delivery == "barrier" {
		f.s.opampClient = qualificationBarrier{f.s.opampClient}
	}
	t.Cleanup(func() {
		if !f.stopped {
			f.stop(t)
		}
	})
	return f
}

func (f *qualificationFixture) start(t *testing.T) {
	t.Helper()
	if err := f.s.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func (f *qualificationFixture) stop(t *testing.T) {
	t.Helper()
	f.stopped = true
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.s.Stop(ctx) }()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Stop: %v", err)
		}
	case <-ctx.Done():
		t.Error("collector Stop exceeded five seconds")
	}
	qualificationEventually(t, func() bool { return !qualificationOpen(f.ingest) && !qualificationOpen(f.health) }, "listeners survived Stop")
}

func (f *qualificationFixture) send(t *testing.T, marker int64) {
	t.Helper()
	qualificationEventually(t, func() bool { return qualificationOpen(f.ingest) }, "real OTLP listener unavailable")
	body := fmt.Sprintf(`{"resourceMetrics":[{"scopeMetrics":[{"metrics":[{"name":"opamp_qualification_marker","gauge":{"dataPoints":[{"asInt":"%d"}]}}]}]}]}`, marker)
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Post("http://"+f.ingest+"/v1/metrics", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("OTLP status %d", resp.StatusCode)
	}
	select {
	case got := <-f.points:
		if got != marker {
			t.Fatalf("sink marker %d != %d", got, marker)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("marker did not reach real exporter destination")
	}
	select {
	case err := <-f.s.Error():
		t.Fatalf("supervisor reported failure: %v", err)
	default:
	}
}

func TestOpAMPIntegrationEarly(t *testing.T) {
	f := qualificationNew(t, "barrier")
	f.start(t)
	if f.s.runningNopConfig.Load() || !qualificationOpen(f.ingest) {
		t.Fatal("EARLY_REMOTE_OVERWRITTEN_BY_NOP_REAL_RUNTIME")
	}
	f.send(t, 1)
	f.p.publish(f.config)
	f.send(t, 2)
}

func TestOpAMPIntegrationLifecycle(t *testing.T) {
	for _, delivery := range []string{"immediate", "delayed"} {
		t.Run(delivery, func(t *testing.T) {
			f := qualificationNew(t, delivery)
			f.start(t)
			if delivery == "delayed" {
				qualificationEventually(t, func() bool { return f.p.connections.Load() > 0 }, "no WebSocket connection")
				if qualificationOpen(f.ingest) || !qualificationOpen(f.health) {
					t.Fatal("disconnected no-op listener boundary incorrect")
				}
				f.p.publish(f.config)
			}
			f.p.awaitStatus(t, f.config, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED)
			f.send(t, 10)
			f.p.disconnect()
			// Existing telemetry continues while control connection recovers.
			f.send(t, 11)
			qualificationEventually(t, func() bool { return f.p.connections.Load() >= 2 }, "WebSocket did not reconnect")
			f.send(t, 12)
			invalid := bytes.Replace(f.config, []byte("receivers: [otlp]"), []byte("receivers: [missing_receiver]"), 1)
			f.p.publish(invalid)
			f.p.awaitStatus(t, invalid, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED)
			f.send(t, 13)
			changed := append(append([]byte(nil), f.config...), []byte("\n# subsequent valid configuration\n")...)
			f.p.publish(changed)
			f.p.awaitStatus(t, changed, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED)
			f.send(t, 14)
			f.stop(t)
		})
	}
}

func TestOpAMPIntegrationTransportFailure(t *testing.T) {
	f := qualificationNew(t, "delayed")
	f.s.managerConfig.ServerEndpoint = ":invalid-url"
	if err := f.s.Start(context.Background()); err == nil {
		t.Fatal("expected real transport Start failure")
	}
	// Start did not succeed, so the owning process would exit rather than call Stop.
	f.stopped = true
	if qualificationOpen(f.ingest) || qualificationOpen(f.health) {
		t.Fatal("listeners leaked after transport startup failure")
	}
}

func TestOpAMPIntegrationUnavailable(t *testing.T) {
	f := qualificationNew(t, "delayed")
	f.s.managerConfig.ServerEndpoint = "ws://" + qualificationAddress(t)
	f.start(t)
	if !qualificationOpen(f.health) || qualificationOpen(f.ingest) {
		t.Fatal("unreachable control server must leave only no-op health listener")
	}
	f.stop(t)
}

func TestOpAMPIntegrationNoopFailure(t *testing.T) {
	f := qualificationNew(t, "delayed")
	occupied, err := net.Listen("tcp", f.health)
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	if err := f.s.Start(context.Background()); err == nil {
		t.Fatal("expected health listener bind failure")
	}
	f.stopped = true
	if f.p.connections.Load() != 0 || qualificationOpen(f.ingest) {
		t.Fatal("transport or OTLP survived failed bootstrap")
	}
}

func qualificationStartupWaiters() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return bytes.Count(buf[:n], []byte("signozcol.(*WrappedCollector).Run.func2("))
}

func TestOpAMPIntegrationFailedReloadWaiters(t *testing.T) {
	// Wait for transient startup pollers from preceding tests to settle first.
	time.Sleep(250 * time.Millisecond)
	before := qualificationStartupWaiters()
	f := qualificationNew(t, "immediate")
	f.start(t)
	f.p.awaitStatus(t, f.config, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED)
	invalid := bytes.Replace(f.config, []byte("receivers: [otlp]"), []byte("receivers: [missing_receiver]"), 1)
	f.p.publish(invalid)
	f.p.awaitStatus(t, invalid, protobufs.RemoteConfigStatuses_RemoteConfigStatuses_FAILED)
	f.send(t, 20)
	f.stop(t)
	qualificationEventually(t, func() bool { return qualificationStartupWaiters() <= before }, "STARTUP_WAITER_LEAK_AFTER_FAILED_RELOAD")
}
