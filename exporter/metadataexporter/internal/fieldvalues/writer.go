// Package fieldvalues writes the field values store of the metadata exporter:
// the pair table signoz_metadata.field_values_sets with its projection, and
// the days on which a field was over its limit in field_days_over_limit.
package fieldvalues

import (
	"context"
	"math"
	"math/rand/v2"
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
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

const (
	minMemoryBytes     = 64 << 20
	maxMemoryBytes     = 1 << 30
	defaultMemoryBytes = 256 << 20
	refreshTimeout     = 30 * time.Second
	// keysFlushInterval is the longest time from the first record of a new
	// key to its insert.
	keysFlushInterval = 15 * time.Second
	keysFlushTimeout  = 30 * time.Second
	// keysStopTimeout bounds the last insert of keys, so a slow server does
	// not hold up the stop of the collector.
	keysStopTimeout = 5 * time.Second
	// startRefreshTimeout bounds the first read of the classifier, which
	// Start waits for.
	startRefreshTimeout = 5 * time.Second
	warnInterval        = time.Minute
	// sharedTimeout bounds each call to the shared cache, so a slow cache
	// cannot hold up the export.
	sharedTimeout = 2 * time.Second
)

// autoMemory counts the writers of the process without max_bytes, which share
// the automatic memory. The collector creates all exporters before the first
// push, so each writer sizes its memory at its first push.
var autoMemory struct {
	sync.Mutex
	writers int
}

// autoPool is the key pool of the writers without max_bytes. Each writer gets
// half of its equal share of key cache as its own floor; the other halves
// make the pool, which the writers draw on as they need keys. The signals of
// a tenant need very different numbers of keys, and the need changes.
var autoPool keyPool

// Settings are what the metadata exporter gives the writer.
type Settings struct {
	Signal    pipeline.Signal
	Conn      driver.Conn
	Logger    *zap.Logger
	Telemetry component.TelemetrySettings
	// ExporterID names the exporter in the telemetry of the key cache.
	ExporterID string
	// Shared is the cache shared by the collectors of a tenant, or nil.
	Shared SharedCache
	// BodyJSON enables the pairs of JSON log bodies, or is nil.
	BodyJSON *BodyJSONLimits
}

// signalState is the state of one signal for the current window and UTC day.
type signalState struct {
	window Window
	day    uint64
	// cache is made at the first push; see autoMemory.
	cache     *keyCache
	budget    budget
	tracker   *tracker
	resources *resourceTable
	// inflight counts the cache keys of batches that are being inserted, so
	// that concurrent batches do not pass the room of the cache together.
	inflight [2]int
	// overFlags holds the fields that this collector saw over their limit by
	// its own count, until the refresh writes their rows.
	overFlags map[fieldID]overField
	keys      *fieldKeys
}

// Writer writes the field values of one signal.
type Writer struct {
	cfg           Config
	signal        pipeline.Signal
	logger        *zap.Logger
	rows          rowWriter
	telemetry     component.TelemetrySettings
	exporterID    string
	shared        SharedCache
	classifier    *classifier
	bodyJSON      *BodyJSONLimits
	alwaysInclude map[string]struct{}
	tel           *telemetry
	now           func() time.Time
	widthMillis   uint64
	spreadMillis  uint64
	autoMemory    bool

	mu      sync.Mutex
	state   *signalState
	class   atomic.Pointer[classification]
	batches sync.Pool

	refreshNow chan struct{}
	keysNow    chan struct{}
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
	rows := &clickhouseWriter{conn: set.Conn, signal: set.Signal.String(), source: cfg.Source, asyncTimeout: cfg.AsyncInsertBusyTimeout}
	w := newWriter(cfg, set, rows, tel)
	w.classifier = &classifier{
		conn:          set.Conn,
		signal:        set.Signal.String(),
		source:        cfg.Source,
		lookbackDays:  cfg.Classification.LookbackDays,
		recordLimit:   cfg.Limits.MaxRecordFieldValues,
		resourceLimit: cfg.Limits.MaxResourceFieldValues,
		maxFields:     cfg.Limits.MaxFieldsPerSignal,
		endedWindow:   cfg.Classification.RefreshInterval,
	}
	return w, nil
}

func newWriter(cfg Config, set Settings, rows rowWriter, tel *telemetry) *Writer {
	alwaysInclude := make(map[string]struct{}, len(cfg.AlwaysInclude))
	for _, name := range cfg.AlwaysInclude {
		alwaysInclude[name] = struct{}{}
	}
	state := &signalState{overFlags: make(map[fieldID]overField), keys: newFieldKeys()}
	places := cfg.Limits.MaxFieldsPerSignal
	if set.Signal == pipeline.SignalMetrics {
		// A metric label has a state for each metric, so the count of the
		// fields does not bound the states. The budget does.
		places = 0
	}
	state.tracker = newTracker(places, &state.budget)
	state.resources = newResourceTable(&state.budget)
	w := &Writer{
		cfg:           cfg,
		signal:        set.Signal,
		logger:        set.Logger,
		rows:          rows,
		telemetry:     set.Telemetry,
		exporterID:    set.ExporterID,
		shared:        set.Shared,
		bodyJSON:      set.BodyJSON,
		alwaysInclude: alwaysInclude,
		tel:           tel,
		now:           time.Now,
		widthMillis:   uint64(cfg.Cache.Window.Milliseconds()),
		spreadMillis:  uint64(cfg.Cache.PreWriteWindow.Milliseconds()),
		state:         state,
		refreshNow:    make(chan struct{}, 1),
		keysNow:       make(chan struct{}, 1),
		stop:          make(chan struct{}),
	}
	w.batches.New = func() any { return newBatchBuffers() }
	if cfg.Cache.MaxBytes == 0 {
		autoMemory.Lock()
		autoMemory.writers++
		autoMemory.Unlock()
		w.autoMemory = true
	}
	return w
}

// autoTotal is the automatic memory of the process: 10% of the Go memory
// limit, between 64 MiB and 1 GiB, or 256 MiB when no limit is set.
func autoTotal() uint64 {
	if limit := debug.SetMemoryLimit(-1); limit > 0 && limit != math.MaxInt64 {
		return min(max(uint64(limit)/10, minMemoryBytes), maxMemoryBytes)
	}
	return defaultMemoryBytes
}

// memoryBytes is the memory of one writer: max_bytes, or an equal share of
// the automatic memory.
func memoryBytes(cfg CacheConfig) uint64 {
	if cfg.MaxBytes > 0 {
		return cfg.MaxBytes
	}
	autoMemory.Lock()
	writers := max(autoMemory.writers, 1)
	autoMemory.Unlock()
	return autoTotal() / uint64(writers)
}

// allocate makes the key cache with three quarters of the memory, and at
// least the smallest buckets of pkg/timebucketedset. The value tracker and
// the resource states get the rest. A writer without max_bytes keeps half of
// its key cache as its floor and shares the other half in autoPool, so its
// cache is sized for its floor and the whole pool. The package allocates as
// keys arrive, and the pool bounds the keys of all writers.
func (w *Writer) allocate() error {
	total := memoryBytes(w.cfg.Cache)
	cacheMemory := total / 4 * 3
	cacheBytes, poolMemory := cacheMemory, uint64(0)
	if w.autoMemory {
		poolMemory = autoTotal() / 8 * 3
		cacheBytes = cacheMemory/2 + poolMemory
	}
	cache, err := newKeyCache(w.cfg.Cache.Window, w.cfg.Cache.PreWriteWindow, cacheBytes, w.cfg.Cache.ReserveShare, w.telemetry,
		attribute.String("exporter", w.exporterID),
		attribute.String("signal", w.signal.String()),
		attribute.String("source", w.cfg.Source))
	if err != nil {
		return err
	}
	if w.autoMemory {
		autoPool.size.Store(int64(windowKeys(poolMemory)))
		cache.share(&autoPool, windowKeys(cacheMemory/2), w.cfg.Cache.ReserveShare)
	}
	w.state.cache = cache
	w.state.budget.limit = int(total - cacheMemory)
	exact, reserve := w.state.cache.capacity()
	w.tel.cacheCapacity[classExact].Store(int64(exact))
	w.tel.cacheCapacity[classReserve].Store(int64(reserve))
	w.tel.trackerCapacity.Store(int64(w.state.budget.limit))
	w.logger.Info("field values memory",
		zap.String("signal", w.signal.String()),
		zap.String("source", w.cfg.Source),
		zap.String("shared", string(w.cfg.Cache.Provider)),
		zap.Uint64("bytes", max(cacheMemory, cacheBuckets*minBucketBytes)+total-cacheMemory),
		zap.Uint64("shared_key_cache_bytes", poolMemory))
	return nil
}

// Start begins the reads of the classifier. It waits for the first read, so
// that the first pushes know the fields over the limit: without the read,
// each such field makes new sets up to its limit on each start of a
// collector.
func (w *Writer) Start() {
	w.wg.Add(1)
	go w.keysLoop()
	if w.classifier == nil {
		return
	}
	w.refresh(startRefreshTimeout)
	w.wg.Add(1)
	go w.refreshLoop()
}

// keysLoop inserts the keys that wait: at each interval, when many wait, and
// at the stop. It is not on the path of a push: the insert waits for the
// server.
func (w *Writer) keysLoop() {
	defer w.wg.Done()
	ticker := time.NewTicker(keysFlushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-w.stop:
			w.flushKeysWithin(keysStopTimeout)
			return
		case <-ticker.C:
		case <-w.keysNow:
		}
		w.flushKeys()
	}
}

func (w *Writer) flushKeys() {
	w.flushKeysWithin(keysFlushTimeout)
}

// flushKeysWithin inserts the keys that wait. A key counts as written only
// when its insert passes; after a failed insert it waits for the next flush.
func (w *Writer) flushKeysWithin(timeout time.Duration) {
	w.mu.Lock()
	now := w.now()
	day := utcDay(now)
	w.state.keys.rotate(day)
	ids, keys := w.state.keys.take()
	notQueued := w.state.keys.notQueued
	w.state.keys.notQueued = 0
	w.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if notQueued > 0 {
		w.tel.keysNotQueued.Add(ctx, int64(notQueued), w.tel.attrs)
	}
	for len(keys) > 0 {
		n := min(len(keys), keysInsertRows)
		if err := w.rows.writeKeys(ctx, keys[:n], now); err != nil {
			w.tel.keyInsertErrors.Add(ctx, 1, w.tel.attrs)
			w.warn("failed to insert field keys", zap.Int("keys", len(keys)), zap.Error(err))
			break
		}
		w.tel.keysWritten.Add(ctx, int64(n), w.tel.attrs)
		w.mu.Lock()
		w.state.keys.confirm(ids[:n], day)
		w.mu.Unlock()
		ids, keys = ids[n:], keys[n:]
	}
	w.mu.Lock()
	w.tel.keysKnown.Store(int64(len(w.state.keys.known)))
	w.tel.keysPending.Store(int64(len(w.state.keys.pending)))
	w.mu.Unlock()
}

func (w *Writer) Shutdown() error {
	w.stopOnce.Do(func() {
		close(w.stop)
		if w.autoMemory {
			autoMemory.Lock()
			autoMemory.writers--
			autoMemory.Unlock()
		}
	})
	w.wg.Wait()
	w.mu.Lock()
	if w.state.cache != nil {
		w.state.cache.shutdown()
	}
	w.mu.Unlock()
	if w.shared != nil {
		return w.shared.Close()
	}
	return nil
}

// refreshLoop runs the classifier at each interval.
// The first interval and the refresh after a day change start after a random
// time, so that the collectors of a tenant do not read together.
func (w *Writer) refreshLoop() {
	defer w.wg.Done()
	interval := w.cfg.Classification.RefreshInterval
	timer := time.NewTimer(rand.N(interval))
	defer timer.Stop()
	for {
		select {
		case <-w.stop:
			return
		case <-timer.C:
			w.refresh(refreshTimeout)
			timer.Reset(interval)
		case <-w.refreshNow:
			// The read for the day that ended is correct only in the first
			// interval of the new day.
			select {
			case <-w.stop:
				return
			case <-time.After(rand.N(interval / 3)):
			}
			w.refresh(refreshTimeout)
		}
	}
}

func (w *Writer) refresh(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	go func() {
		select {
		case <-w.stop:
			cancel()
		case <-ctx.Done():
		}
	}()
	w.mu.Lock()
	own := make([]overField, 0, len(w.state.overFlags))
	for _, f := range w.state.overFlags {
		own = append(own, f)
	}
	w.mu.Unlock()
	next, writeErr, err := w.classifier.refresh(ctx, w.class.Load(), w.now(), own)
	if err != nil {
		w.tel.classifierErrors.Add(ctx, 1, w.tel.attrs)
		w.warn("failed to read the fields over the limit", zap.Error(err))
		return
	}
	if writeErr != nil {
		w.tel.classifierErrors.Add(ctx, 1, w.tel.attrs)
		w.warn("failed to write the fields over the limit", zap.Error(writeErr))
	}
	w.class.Store(next)
	w.tel.classifiedAt.Store(w.now().Unix())
	w.mu.Lock()
	if writeErr == nil {
		for _, f := range own {
			id := f.id()
			if w.state.overFlags[id].day == f.day {
				delete(w.state.overFlags, id)
			}
		}
	}
	w.state.tracker.place(next.known)
	w.mu.Unlock()
}

func utcDay(t time.Time) uint64 {
	return uint64(t.Unix() / 86400)
}

func (w *Writer) WriteLogs(ctx context.Context, ld plog.Logs) error {
	in := getRecordsInput()
	defer putRecordsInput(in)
	in.addLogs(ld, w.bodyJSON)
	return w.push(ctx, func(b *batch) { b.addRecords(in) })
}

func (w *Writer) WriteTraces(ctx context.Context, td ptrace.Traces) error {
	in := getRecordsInput()
	defer putRecordsInput(in)
	in.addTraces(td)
	return w.push(ctx, func(b *batch) { b.addRecords(in) })
}

func (w *Writer) WriteMetrics(ctx context.Context, md pmetric.Metrics) error {
	in := getMetricsInput()
	defer putMetricsInput(in)
	in.addMetrics(md)
	return w.push(ctx, func(b *batch) { b.addSeries(in) })
}

// push builds the rows of one export call, drops the rows that another
// collector already wrote in the window, inserts the rest, and caches the
// keys only when the insert succeeds. A failed insert is not retried: its
// rows are written again at the next sighting of their sets. push never
// returns an error, so the metadata never holds up the export of the data.
func (w *Writer) push(ctx context.Context, fill func(*batch)) error {
	w.mu.Lock()
	if w.state.cache == nil {
		if err := w.allocate(); err != nil {
			w.mu.Unlock()
			w.warn("failed to create the field values key cache", zap.Error(err))
			return nil
		}
	}
	b := w.newBatch()
	fill(b)
	st := w.state
	st.inflight[classExact] += b.pendingCount[classExact]
	st.inflight[classReserve] += b.pendingCount[classReserve]
	manyKeys := len(st.keys.pending) >= keysFlushAt
	w.mu.Unlock()
	if manyKeys {
		select {
		case w.keysNow <- struct{}{}:
		default:
		}
	}
	defer w.release(b)
	w.tel.recordBatch(ctx, b.stats)

	rows, sharedKeys := w.dropShared(ctx, b)
	var err error
	if len(rows) > 0 {
		err = w.rows.write(ctx, rows)
	}

	w.mu.Lock()
	st.inflight[classExact] -= b.pendingCount[classExact]
	st.inflight[classReserve] -= b.pendingCount[classReserve]
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
	windows := b.windows()
	for g, keys := range sharedKeys {
		if len(keys) == 0 {
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, sharedTimeout)
		err := w.shared.Add(sctx, windows[g], keys)
		cancel()
		if err != nil {
			w.tel.sharedCacheErrors.Add(ctx, 1, w.tel.attrs)
			w.logger.Debug("failed to add keys to the shared cache", zap.Error(err))
		}
	}
	return nil
}

// windows gives the window of the batch and the window before it, which is
// still the window of the keys whose own window did not start.
func (b *batch) windows() [2]Window {
	width := b.window.End - b.window.Start
	return [2]Window{b.window, {Start: b.window.Start - width, End: b.window.Start}}
}

// dropShared removes the rows whose keys the shared cache already has in their
// window, and returns the keys to add to the shared cache after the insert,
// for each of the two windows. On an error of the shared cache, all rows are
// kept.
func (w *Writer) dropShared(ctx context.Context, b *batch) ([]row, [2][]uint64) {
	var add [2][]uint64
	if w.shared == nil {
		return b.rows, add
	}
	var pending [2][]uint64
	for k := range b.pending {
		g := 0
		if b.state.cache.early(k, b.window.Start, b.nowMillis) {
			g = 1
		}
		pending[g] = append(pending[g], k)
	}
	windows := b.windows()
	var drop map[int]struct{}
	for g, keys := range pending {
		if len(keys) == 0 {
			continue
		}
		sctx, cancel := context.WithTimeout(ctx, sharedTimeout)
		seen, err := w.shared.Seen(sctx, windows[g], keys)
		cancel()
		if err != nil {
			w.tel.sharedCacheErrors.Add(ctx, 1, w.tel.attrs)
			w.logger.Debug("failed to read the shared cache", zap.Error(err))
			add[g] = keys
			continue
		}
		for i, k := range keys {
			if !seen[i] {
				add[g] = append(add[g], k)
				continue
			}
			for _, r := range b.owners[k] {
				if drop == nil {
					drop = make(map[int]struct{})
				}
				drop[r] = struct{}{}
			}
		}
	}
	if len(drop) == 0 {
		return b.rows, add
	}
	rows := make([]row, 0, len(b.rows)-len(drop))
	for i, r := range b.rows {
		if _, ok := drop[i]; !ok {
			rows = append(rows, r)
		}
	}
	w.tel.rowsSkippedShared.Add(ctx, int64(len(drop)), w.tel.attrs)
	return rows, add
}

// newBatch moves the state to the current window and day, and returns an
// empty batch.
func (w *Writer) newBatch() *batch {
	now := uint64(w.now().UnixMilli())
	st := w.state
	if start := now / w.widthMillis * w.widthMillis; st.window.End == 0 || start != st.window.Start {
		st.window = Window{Start: start, End: start + w.widthMillis}
		st.cache.rotate(st.window.Start)
		st.resources.reset()
		if day := now / dayMillis; day != st.day {
			st.day = day
			st.keys.rotate(day)
			var known []fieldID
			if class := w.class.Load(); class != nil {
				known = class.known
			}
			st.tracker.reset(known)
			select {
			case w.refreshNow <- struct{}{}:
			default:
			}
		}
	}
	b := w.batches.Get().(*batch)
	b.window = st.window
	b.dayStart = st.day * dayMillis
	b.limits = w.cfg.Limits
	b.alwaysInclude = w.alwaysInclude
	b.state = st
	b.class = w.class.Load()
	b.nowMillis = now
	b.spreadMillis = w.spreadMillis
	b.metrics = w.signal == pipeline.SignalMetrics
	return b
}

func (w *Writer) release(b *batch) {
	if cap(b.rows) > maxPooledRows {
		return
	}
	b.reset()
	w.batches.Put(b)
}

// commit caches the keys of a written batch.
func (w *Writer) commit(b *batch) {
	st := w.state
	st.cache.apply(b.window.Start, b.nowMillis, b.pending)
	for rs, c := range b.steps {
		rs.stepSets += c.sets
		rs.stepPairs += c.pairs
	}
	w.tel.cacheUsed[classExact].Store(int64(st.cache.used[classExact]))
	w.tel.cacheUsed[classReserve].Store(int64(st.cache.used[classReserve]))
	w.tel.trackerUsed.Store(int64(st.budget.used))
}

// rollback gives back the sample budget of a failed batch.
func (w *Writer) rollback(b *batch) {
	if b.dayStart != w.state.day*dayMillis {
		return
	}
	for _, s := range b.samples {
		s.st.values.remove(s.vh)
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
