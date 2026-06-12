package signozclickhousemetrics

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"

	pkgfingerprint "github.com/SigNoz/signoz-otel-collector/internal/common/fingerprint"
)

// protectedLabels can never be aggregated away: le and quantile are series
// identity for histogram buckets and summary quantiles, the dunder keys are
// internal markers baked into every fingerprint, and deployment.environment
// is a physical column and part of the sharding key.
var protectedLabels = map[string]struct{}{
	"le":                     {},
	"quantile":               {},
	"__name__":               {},
	"__temporality__":        {},
	"deployment.environment": {},
}

const rulesPollTimeout = 30 * time.Second

// reductionRule is one compiled label-drop rule, keyed by the base metric
// name; it covers every series derived from that metric (.count, .sum, .min,
// .max, .bucket, .quantile).
type reductionRule struct {
	dropKeys map[string]struct{}
	// effectiveFromUnixMilli is compared against the datapoint timestamp, not
	// the wall clock: every collector replica then starts reducing at the same
	// data-time boundary regardless of when it polled the rule, so no 60s
	// bucket is ever partially reduced.
	effectiveFromUnixMilli int64
}

func (r *reductionRule) drop(key string) bool {
	_, ok := r.dropKeys[key]
	return ok
}

func (r *reductionRule) appliesAt(unixMilli int64) bool {
	return unixMilli >= r.effectiveFromUnixMilli
}

type ruleSet map[string]*reductionRule

// ruleFor returns the active rule for the base metric name, or nil.
func (c *clickhouseMetricsExporter) ruleFor(metricName string) *reductionRule {
	rules := c.reductionRules.Load()
	if rules == nil {
		return nil
	}
	return (*rules)[metricName]
}

// pollReductionRules refreshes the in-memory ruleset from ClickHouse. It
// fails open: on error the last known ruleset stays active, and a series
// without an applicable rule is written unreduced, which is always correct,
// just at full fidelity.
func (c *clickhouseMetricsExporter) pollReductionRules(ctx context.Context) {
	rules, err := c.fetchReductionRules(ctx)
	if err != nil {
		c.logger.Error("failed to refresh metric reduction rules; keeping last known rules", zap.Error(err))
		if c.reductionPollErrors != nil {
			c.reductionPollErrors.Add(ctx, 1)
		}
		return
	}
	c.reductionRules.Store(&rules)
	if c.reductionActiveRules != nil {
		c.reductionActiveRules.Record(ctx, int64(len(rules)))
	}
}

func (c *clickhouseMetricsExporter) fetchReductionRules(ctx context.Context) (ruleSet, error) {
	// the table is tiny and append-only per (metric_name, updated_at); argMax
	// over the distributed table picks the latest version of each rule no
	// matter which shard its rows landed on
	query := fmt.Sprintf(`SELECT
			metric_name,
			argMax(drop_labels, updated_at) AS drop_labels,
			argMax(effective_from_unix_milli, updated_at) AS effective_from_unix_milli,
			argMax(deleted, updated_at) AS deleted
		FROM %s.%s
		GROUP BY metric_name`, c.cfg.Database, c.cfg.Reduction.RulesTable)

	rows, err := c.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			c.logger.Warn("failed to close reduction rules result set", zap.Error(err))
		}
	}()

	rules := make(ruleSet)
	for rows.Next() {
		var (
			metricName    string
			dropLabels    []string
			effectiveFrom int64
			deleted       bool
		)
		if err := rows.Scan(&metricName, &dropLabels, &effectiveFrom, &deleted); err != nil {
			return nil, err
		}
		if deleted || len(dropLabels) == 0 {
			continue
		}
		dropKeys := make(map[string]struct{}, len(dropLabels))
		valid := true
		for _, label := range dropLabels {
			if _, protected := protectedLabels[label]; protected {
				c.logger.Warn("ignoring reduction rule that drops a protected label",
					zap.String("metric_name", metricName), zap.String("label", label))
				valid = false
				break
			}
			dropKeys[label] = struct{}{}
		}
		if !valid {
			continue
		}
		rules[metricName] = &reductionRule{
			dropKeys:               dropKeys,
			effectiveFromUnixMilli: effectiveFrom,
		}
	}
	return rules, rows.Err()
}

func (c *clickhouseMetricsExporter) startReductionRulesPoller() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.cfg.Reduction.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.closeChan:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), rulesPollTimeout)
				c.pollReductionRules(ctx)
				cancel()
			}
		}
	}()
}

// reducer computes reduced fingerprints and series for the datapoints of one
// metric under one rule. The reduced resource and scope fingerprints are
// computed lazily once and reused for every datapoint of the metric.
type reducer struct {
	rule                *reductionRule
	resourceFingerprint *pkgfingerprint.Fingerprint
	scopeFingerprint    *pkgfingerprint.Fingerprint

	reducedResource *pkgfingerprint.Fingerprint
	reducedScope    *pkgfingerprint.Fingerprint
	seen            map[uint64]struct{}
}

// firstSeen reports whether this reduced fingerprint is new within the batch:
// consecutive datapoints of one series all map to the same reduced series, so
// one reduced series row per batch is enough (the hourly write cache dedups
// across batches anyway) and building the labels JSON per point is wasted.
func (r *reducer) firstSeen(fingerprint uint64) bool {
	if r.seen == nil {
		r.seen = make(map[uint64]struct{})
	}
	if _, ok := r.seen[fingerprint]; ok {
		return false
	}
	r.seen[fingerprint] = struct{}{}
	return true
}

// reducedSeries carries everything a call site needs to write the reduced
// sample column and the reduced series row.
type reducedSeries struct {
	fingerprint uint64
	point       *pkgfingerprint.Fingerprint
	scope       *pkgfingerprint.Fingerprint
	resource    *pkgfingerprint.Fingerprint
}

// newReducerFor returns a reducer for the metric, or nil when reduction is
// disabled or no rule matches the base metric name.
func (c *clickhouseMetricsExporter) newReducerFor(metricName string, resourceFingerprint, scopeFingerprint *pkgfingerprint.Fingerprint) *reducer {
	if !c.cfg.Reduction.Enabled {
		return nil
	}
	rule := c.ruleFor(metricName)
	if rule == nil {
		return nil
	}
	return &reducer{
		rule:                rule,
		resourceFingerprint: resourceFingerprint,
		scopeFingerprint:    scopeFingerprint,
	}
}

// reduce returns the reduced series for a datapoint fingerprint, or nil when
// the rule does not apply at the datapoint's timestamp (pre-epoch data stays
// unreduced and flows to the long-retention tables, where pre-epoch queries
// look for it). nameWithSuffix is the stored series name, e.g. metric.bucket.
func (r *reducer) reduce(pointFingerprint *pkgfingerprint.Fingerprint, nameWithSuffix string, unixMilli int64) *reducedSeries {
	if r == nil || !r.rule.appliesAt(unixMilli) {
		return nil
	}
	if r.reducedResource == nil {
		r.reducedResource = r.resourceFingerprint.Reduced(pkgfingerprint.InitialOffset, r.rule.drop)
		r.reducedScope = r.scopeFingerprint.Reduced(r.reducedResource.Hash(), r.rule.drop)
	}
	reducedPoint := pointFingerprint.Reduced(r.reducedScope.Hash(), r.rule.drop)
	return &reducedSeries{
		fingerprint: reducedPoint.HashWithName(nameWithSuffix),
		point:       reducedPoint,
		scope:       r.reducedScope,
		resource:    r.reducedResource,
	}
}

func (r *reducedSeries) fingerprintOrZero() uint64 {
	if r == nil {
		return 0
	}
	return r.fingerprint
}

// reducedTsFrom builds the reduced series row from the raw one: same series
// metadata, but the reduced fingerprint as identity and the remaining label
// set as labels/attrs.
func reducedTsFrom(raw *ts, reduced *reducedSeries) *ts {
	pointMap := reduced.point.AttributesAsMap()
	scopeMap := reduced.scope.AttributesAsMap()
	resourceMap := reduced.resource.AttributesAsMap()
	return &ts{
		env:                raw.env,
		temporality:        raw.temporality,
		metricName:         raw.metricName,
		description:        raw.description,
		unit:               raw.unit,
		typ:                raw.typ,
		isMonotonic:        raw.isMonotonic,
		fingerprint:        reduced.fingerprint,
		reducedFingerprint: reduced.fingerprint,
		isReduced:          true,
		unixMilli:          raw.unixMilli,
		labels:             pkgfingerprint.NewLabelsAsJSONString(raw.metricName, pointMap, scopeMap, resourceMap),
		attrs:              pointMap,
		scopeAttrs:         scopeMap,
		resourceAttrs:      resourceMap,
	}
}
