// Package fieldvalues writes the field values store of the metadata exporter:
// the pair table signoz_metadata.field_values_sets, from which the
// field_values_daily view is built. Plain values, related values and metric
// keys are read from these two tables.
package fieldvalues

import (
	"context"
	"math"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
	"go.uber.org/zap"
)

const (
	minCacheBytes     = 64 << 20
	maxCacheBytes     = 1 << 30
	defaultCacheBytes = 256 << 20
	// signalsPerProcess divides the automatic cache size, because each of the
	// three signals has its own writer and cache.
	signalsPerProcess = 3
	refreshTimeout    = 30 * time.Second
	warnInterval      = time.Minute
	// sharedTimeout bounds each call to the shared cache, so a slow cache
	// cannot hold up the export.
	sharedTimeout = 2 * time.Second
)

// Settings are what the metadata exporter gives the writer.
type Settings struct {
	Signal    pipeline.Signal
	Conn      driver.Conn
	Logger    *zap.Logger
	Telemetry component.TelemetrySettings
	// Shared is the cache shared by the collectors of a tenant, or nil.
	Shared SharedCache
	// BodyJSON enables the pairs of JSON log bodies, or is nil.
	BodyJSON *BodyJSONLimits
}

// signalState is the state of one signal for the current UTC day.
type signalState struct {
	day       uint64
	cache     *dayCache
	tracker   *tracker
	resources map[uint64]*resourceState
	// inflight counts the cache keys of batches that are being inserted, so
	// that concurrent batches do not pass the room of the cache together.
	inflight [2]int
}

func (s *signalState) rotate(day uint64, class *classification) {
	s.day = day
	s.cache.rotate(day)
	var known []fieldKey
	if class != nil {
		known = class.known
	}
	s.tracker.reset(known)
	s.resources = make(map[uint64]*resourceState)
}

// Writer writes the field values of one signal.
type Writer struct {
	cfg           Config
	signal        pipeline.Signal
	logger        *zap.Logger
	rows          rowWriter
	shared        SharedCache
	classifier    *classifier
	bodyJSON      *BodyJSONLimits
	alwaysInclude map[string]struct{}
	tel           *telemetry
	now           func() time.Time

	mu    sync.Mutex
	state *signalState
	class atomic.Pointer[classification]

	refreshNow chan struct{}
	stop       chan struct{}
	stopOnce   sync.Once
	wg         sync.WaitGroup

	// lastWarn limits the warnings of failed inserts to one per warnInterval,
	// so a missing table cannot flood the log. Telemetry counts every failure.
	lastWarn atomic.Int64
}

func New(cfg Config, set Settings) (*Writer, error) {
	tel, err := newTelemetry(set.Telemetry, set.Signal.String(), cfg.Source)
	if err != nil {
		return nil, err
	}
	rows := &clickhouseWriter{conn: set.Conn, signal: set.Signal.String(), source: cfg.Source}
	w := newWriter(cfg, set, rows, tel)
	w.classifier = &classifier{
		conn:          set.Conn,
		signal:        set.Signal.String(),
		source:        cfg.Source,
		lookbackDays:  cfg.Classification.LookbackDays,
		recordLimit:   cfg.Limits.MaxRecordFieldValues,
		resourceLimit: cfg.Limits.MaxResourceFieldValues,
		maxFields:     cfg.Limits.MaxFieldsPerSignal,
	}
	return w, nil
}

func newWriter(cfg Config, set Settings, rows rowWriter, tel *telemetry) *Writer {
	alwaysInclude := make(map[string]struct{}, len(cfg.AlwaysInclude))
	for _, name := range cfg.AlwaysInclude {
		alwaysInclude[name] = struct{}{}
	}
	cacheMemory := cacheBytes(cfg.Cache)
	set.Logger.Info("field values day cache",
		zap.String("signal", set.Signal.String()),
		zap.String("source", cfg.Source),
		zap.String("shared", string(cfg.Cache.Provider)),
		zap.Uint64("bytes", cacheMemory))
	w := &Writer{
		cfg:           cfg,
		signal:        set.Signal,
		logger:        set.Logger,
		rows:          rows,
		shared:        set.Shared,
		bodyJSON:      set.BodyJSON,
		alwaysInclude: alwaysInclude,
		tel:           tel,
		now:           time.Now,
		state: &signalState{
			cache:     newDayCache(cacheMemory, cfg.Cache.ReserveShare),
			tracker:   newTracker(cfg.Limits.MaxFieldsPerSignal),
			resources: make(map[uint64]*resourceState),
		},
		refreshNow: make(chan struct{}, 1),
		stop:       make(chan struct{}),
	}
	exact, reserve := w.state.cache.capacity()
	tel.cacheCapacity[classExact].Store(int64(exact))
	tel.cacheCapacity[classReserve].Store(int64(reserve))
	return w
}

// cacheBytes is the configured size of the cache of one signal. Without a
// size, the three signals share 10% of the Go memory limit, between 64 MiB and
// 1 GiB, or 256 MiB when no limit is set.
func cacheBytes(cfg CacheConfig) uint64 {
	if cfg.MaxBytes > 0 {
		return cfg.MaxBytes
	}
	total := uint64(defaultCacheBytes)
	if limit := debug.SetMemoryLimit(-1); limit > 0 && limit != math.MaxInt64 {
		total = min(max(uint64(limit)/10, minCacheBytes), maxCacheBytes)
	}
	return total / signalsPerProcess
}

// Start begins the reads of field_values_daily. Metrics have no value limits,
// so they need no reads.
func (w *Writer) Start() {
	if w.signal == pipeline.SignalMetrics || w.classifier == nil {
		return
	}
	w.wg.Add(1)
	go w.refreshLoop()
}

func (w *Writer) Shutdown() error {
	w.stopOnce.Do(func() { close(w.stop) })
	w.wg.Wait()
	if w.shared != nil {
		return w.shared.Close()
	}
	return nil
}

func (w *Writer) refreshLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(w.cfg.Classification.RefreshInterval)
	defer ticker.Stop()
	w.refresh()
	for {
		select {
		case <-w.stop:
			return
		case <-ticker.C:
			w.refresh()
		case <-w.refreshNow:
			w.refresh()
		}
	}
}

func (w *Writer) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), refreshTimeout)
	defer cancel()
	go func() {
		select {
		case <-w.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	next, err := w.classifier.refresh(ctx, w.class.Load(), utcDay(w.now()))
	if err != nil {
		w.warn("failed to read field_values_daily", zap.Error(err))
		return
	}
	w.class.Store(next)
	w.mu.Lock()
	w.state.tracker.place(next.known)
	w.mu.Unlock()
}

func utcDay(t time.Time) uint64 {
	return uint64(t.UTC().Unix() / 86400)
}

func (w *Writer) WriteLogs(ctx context.Context, ld plog.Logs) error {
	return w.push(ctx, func(b *batch) { b.addLogs(ld) })
}

func (w *Writer) WriteTraces(ctx context.Context, td ptrace.Traces) error {
	return w.push(ctx, func(b *batch) { b.addTraces(td) })
}

func (w *Writer) WriteMetrics(ctx context.Context, md pmetric.Metrics) error {
	return w.push(ctx, func(b *batch) { b.addMetrics(md) })
}

// push builds the rows of one export call, drops the rows that another
// collector already wrote today, inserts the rest, and caches the keys only
// when the insert succeeds. A failed insert is not retried: its rows are
// written again at the next sighting of their sets. push never returns an
// error, so the metadata never holds up the export of the data.
func (w *Writer) push(ctx context.Context, fill func(*batch)) error {
	w.mu.Lock()
	b := w.newBatch()
	fill(b)
	w.state.inflight[classExact] += b.pendingCount[classExact]
	w.state.inflight[classReserve] += b.pendingCount[classReserve]
	w.mu.Unlock()
	w.tel.recordBatch(ctx, b.stats)

	rows, sharedKeys := w.dropShared(ctx, b)
	var err error
	if len(rows) > 0 {
		err = w.rows.write(ctx, rows)
	}

	w.mu.Lock()
	w.state.inflight[classExact] -= b.pendingCount[classExact]
	w.state.inflight[classReserve] -= b.pendingCount[classReserve]
	if err == nil {
		w.commit(b)
	} else {
		w.rollback(b)
	}
	w.mu.Unlock()

	if err != nil {
		w.tel.insertErrors.Add(ctx, 1, w.tel.attrs)
		w.warn("failed to insert field values", zap.Int("rows", len(rows)), zap.Error(err))
		return nil
	}
	w.tel.rowsWritten.Add(ctx, int64(len(rows)), w.tel.attrs)
	if len(sharedKeys) > 0 {
		sctx, cancel := context.WithTimeout(ctx, sharedTimeout)
		defer cancel()
		if err := w.shared.Add(sctx, b.day, sharedKeys); err != nil {
			w.tel.sharedCacheErrors.Add(ctx, 1, w.tel.attrs)
			w.logger.Debug("failed to add keys to the shared cache", zap.Error(err))
		}
	}
	return nil
}

// dropShared removes the rows whose keys the shared cache already has today,
// and returns the keys to add to the shared cache after the insert. On an
// error of the shared cache, all rows are kept.
func (w *Writer) dropShared(ctx context.Context, b *batch) ([]row, []uint64) {
	if w.shared == nil || len(b.pending) == 0 {
		return b.rows, nil
	}
	keys := make([]uint64, 0, len(b.pending))
	for k := range b.pending {
		keys = append(keys, k)
	}
	sctx, cancel := context.WithTimeout(ctx, sharedTimeout)
	defer cancel()
	seen, err := w.shared.Seen(sctx, b.day, keys)
	if err != nil {
		w.tel.sharedCacheErrors.Add(ctx, 1, w.tel.attrs)
		w.logger.Debug("failed to read the shared cache", zap.Error(err))
		return b.rows, keys
	}
	drop := make(map[int]struct{})
	unseen := keys[:0:0]
	for i, k := range keys {
		if !seen[i] {
			unseen = append(unseen, k)
			continue
		}
		for _, r := range b.owners[k] {
			drop[r] = struct{}{}
		}
	}
	if len(drop) == 0 {
		return b.rows, unseen
	}
	rows := make([]row, 0, len(b.rows)-len(drop))
	for i, r := range b.rows {
		if _, ok := drop[i]; !ok {
			rows = append(rows, r)
		}
	}
	w.tel.rowsSkippedShared.Add(ctx, int64(len(drop)), w.tel.attrs)
	return rows, unseen
}

func (w *Writer) newBatch() *batch {
	now := w.now()
	if day := utcDay(now); day != w.state.day {
		w.state.rotate(day, w.class.Load())
		select {
		case w.refreshNow <- struct{}{}:
		default:
		}
	}
	return &batch{
		day:           w.state.day,
		limits:        w.cfg.Limits,
		alwaysInclude: w.alwaysInclude,
		bodyLimits:    w.bodyJSON,
		state:         w.state,
		class:         w.class.Load(),
		nowMillis:     uint64(now.UnixMilli()),
		pending:       make(map[uint64]cacheClass),
		owners:        make(map[uint64][]int),
		stats:         batchStats{leftOut: make(map[leftOutReason]int)},
	}
}

// commit caches the keys of a written batch. A batch of an earlier day caches
// nothing, so its sets are written again on the new day.
func (w *Writer) commit(b *batch) {
	if b.day != w.state.day {
		return
	}
	before := w.state.cache.collisions
	for k, class := range b.pending {
		w.state.cache.insert(k, class)
	}
	if n := w.state.cache.collisions - before; n > 0 {
		w.tel.cacheCollisions.Add(context.Background(), int64(n), w.tel.attrs)
	}
	w.tel.cacheUsed[classExact].Store(int64(w.state.cache.used[classExact]))
	w.tel.cacheUsed[classReserve].Store(int64(w.state.cache.used[classReserve]))
}

// rollback gives back the sample budget of a failed batch.
func (w *Writer) rollback(b *batch) {
	if b.day != w.state.day {
		return
	}
	for _, s := range b.samples {
		delete(s.st.values, s.vh)
	}
}

func (w *Writer) warn(msg string, fields ...zap.Field) {
	now := time.Now().UnixNano()
	last := w.lastWarn.Load()
	if now-last < int64(warnInterval) || !w.lastWarn.CompareAndSwap(last, now) {
		return
	}
	w.logger.Warn(msg, append(fields, zap.String("signal", w.signal.String()))...)
}
