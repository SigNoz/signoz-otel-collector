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
	minMemoryBytes     = 64 << 20
	maxMemoryBytes     = 1 << 30
	defaultMemoryBytes = 256 << 20
	refreshTimeout     = 30 * time.Second
	warnInterval       = time.Minute
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

// signalState is the state of one signal for the current window and UTC day.
type signalState struct {
	window      Window
	windowIndex uint64
	day         uint64
	// cache is made at the first push; see autoMemory.
	cache     *windowCache
	budget    budget
	tracker   *tracker
	resources *resourceTable
	// inflight and inflightAhead count the cache keys of batches that are
	// being inserted, so that concurrent batches do not pass the room of the
	// cache together.
	inflight      [2]int
	inflightAhead int
}

// Writer writes the field values of one signal.
type Writer struct {
	cfg            Config
	signal         pipeline.Signal
	logger         *zap.Logger
	rows           rowWriter
	shared         SharedCache
	classifier     *classifier
	bodyJSON       *BodyJSONLimits
	alwaysInclude  map[string]struct{}
	tel            *telemetry
	now            func() time.Time
	widthMillis    uint64
	preWriteMillis uint64
	autoMemory     bool

	mu      sync.Mutex
	state   *signalState
	class   atomic.Pointer[classification]
	batches sync.Pool

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
	state := &signalState{}
	state.tracker = newTracker(cfg.Limits.MaxFieldsPerSignal, &state.budget)
	state.resources = newResourceTable(&state.budget)
	w := &Writer{
		cfg:            cfg,
		signal:         set.Signal,
		logger:         set.Logger,
		rows:           rows,
		shared:         set.Shared,
		bodyJSON:       set.BodyJSON,
		alwaysInclude:  alwaysInclude,
		tel:            tel,
		now:            time.Now,
		widthMillis:    uint64(cfg.Cache.Window.Milliseconds()),
		preWriteMillis: uint64(cfg.Cache.PreWriteWindow.Milliseconds()),
		state:          state,
		refreshNow:     make(chan struct{}, 1),
		stop:           make(chan struct{}),
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

// memoryBytes is the memory of one writer: max_bytes, or its share of 10% of
// the Go memory limit, between 64 MiB and 1 GiB, or of 256 MiB when no limit
// is set.
func memoryBytes(cfg CacheConfig) uint64 {
	if cfg.MaxBytes > 0 {
		return cfg.MaxBytes
	}
	total := uint64(defaultMemoryBytes)
	if limit := debug.SetMemoryLimit(-1); limit > 0 && limit != math.MaxInt64 {
		total = min(max(uint64(limit)/10, minMemoryBytes), maxMemoryBytes)
	}
	autoMemory.Lock()
	writers := max(autoMemory.writers, 1)
	autoMemory.Unlock()
	return total / uint64(writers)
}

// allocate makes the window cache with three quarters of the memory. The
// value tracker and the resource states get the rest.
func (w *Writer) allocate() {
	total := memoryBytes(w.cfg.Cache)
	cacheMemory := total / 4 * 3
	w.state.cache = newWindowCache(cacheMemory, w.cfg.Cache.ReserveShare)
	w.state.budget.limit = int(total - cacheMemory)
	exact, reserve := w.state.cache.capacity()
	w.tel.cacheCapacity[classExact].Store(int64(exact))
	w.tel.cacheCapacity[classReserve].Store(int64(reserve))
	w.tel.cacheCapacity[partAhead].Store(int64(exact))
	w.tel.trackerCapacity.Store(int64(w.state.budget.limit))
	w.logger.Info("field values memory",
		zap.String("signal", w.signal.String()),
		zap.String("source", w.cfg.Source),
		zap.String("shared", string(w.cfg.Cache.Provider)),
		zap.Uint64("bytes", total))
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
	w.stopOnce.Do(func() {
		close(w.stop)
		if w.autoMemory {
			autoMemory.Lock()
			autoMemory.writers--
			autoMemory.Unlock()
		}
	})
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
	b := w.newBatch()
	fill(b)
	st := w.state
	st.inflight[classExact] += b.pendingCount[classExact]
	st.inflight[classReserve] += b.pendingCount[classReserve]
	st.inflightAhead += len(b.pendingAhead)
	w.mu.Unlock()
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
	st.inflightAhead -= len(b.pendingAhead)
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

// windows gives the window of the batch and the next window.
func (b *batch) windows() [2]Window {
	width := b.window.End - b.window.Start
	return [2]Window{b.window, {Start: b.window.End, End: b.window.End + width}}
}

// dropShared removes the rows whose keys the shared cache already has in their
// window, and returns the keys to add to the shared cache after the insert,
// for the window and the next window. On an error of the shared cache, all
// rows are kept.
func (w *Writer) dropShared(ctx context.Context, b *batch) ([]row, [2][]uint64) {
	var add [2][]uint64
	if w.shared == nil {
		return b.rows, add
	}
	windows := b.windows()
	var drop map[int]struct{}
	for g := range windows {
		keys := pendingKeys(b, g)
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
			for _, r := range b.owners[g][k] {
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

func pendingKeys(b *batch, group int) []uint64 {
	if group == 0 {
		keys := make([]uint64, 0, len(b.pending))
		for k := range b.pending {
			keys = append(keys, k)
		}
		return keys
	}
	keys := make([]uint64, 0, len(b.pendingAhead))
	for k := range b.pendingAhead {
		keys = append(keys, k)
	}
	return keys
}

// newBatch moves the state to the current window and day, and returns an
// empty batch.
func (w *Writer) newBatch() *batch {
	now := uint64(w.now().UnixMilli())
	st := w.state
	if st.cache == nil {
		w.allocate()
	}
	if idx := now / w.widthMillis; st.window.End == 0 || idx != st.windowIndex {
		st.windowIndex = idx
		st.window = Window{Start: idx * w.widthMillis, End: (idx + 1) * w.widthMillis}
		st.cache.rotate(idx)
		st.resources.reset()
		if day := now / dayMillis; day != st.day {
			st.day = day
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
	b.windowIndex = st.windowIndex
	b.dayStart = st.day * dayMillis
	b.limits = w.cfg.Limits
	b.alwaysInclude = w.alwaysInclude
	b.state = st
	b.class = w.class.Load()
	b.nowMillis = now
	b.preWriteMillis = w.preWriteMillis
	return b
}

func (w *Writer) release(b *batch) {
	b.reset()
	w.batches.Put(b)
}

// commit caches the keys of a written batch. When the window changed while
// the batch was inserted, its keys written ahead are the keys of the new
// window; a batch of an older window caches nothing.
func (w *Writer) commit(b *batch) {
	st := w.state
	before := st.cache.collisions
	switch b.windowIndex {
	case st.windowIndex:
		for k, class := range b.pending {
			st.cache.insert(k, class)
		}
		for k := range b.pendingAhead {
			st.cache.insertNext(k)
		}
	case st.windowIndex - 1:
		for k := range b.pendingAhead {
			st.cache.insert(k, classExact)
		}
	default:
		return
	}
	if n := st.cache.collisions - before; n > 0 {
		w.tel.cacheCollisions.Add(context.Background(), int64(n), w.tel.attrs)
	}
	w.tel.cacheUsed[classExact].Store(int64(st.cache.used[classExact]))
	w.tel.cacheUsed[classReserve].Store(int64(st.cache.used[classReserve]))
	w.tel.cacheUsed[partAhead].Store(int64(st.cache.nextUsed))
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
