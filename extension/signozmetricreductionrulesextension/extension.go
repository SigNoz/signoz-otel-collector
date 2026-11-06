package signozmetricreductionrulesextension

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/extension/signozmetricreductionrulesextension/internal/metadata"
	"github.com/SigNoz/signoz-otel-collector/pkg/metricreductionrule"
)

const rulesPollTimeout = 30 * time.Second

type rulesExtension struct {
	cfg       *Config
	logger    *zap.Logger
	telemetry *metadata.TelemetryBuilder
	conn      clickhouse.Conn
	wg        sync.WaitGroup

	reductionRules atomic.Pointer[metricreductionrule.RuleSet]

	closeChan chan struct{}
}

var _ metricreductionrule.Source = (*rulesExtension)(nil)

func newExtension(cfg *Config, settings component.TelemetrySettings, conn clickhouse.Conn) (*rulesExtension, error) {
	telemetry, err := metadata.NewTelemetryBuilder(settings)
	if err != nil {
		return nil, err
	}
	return &rulesExtension{
		cfg:       cfg,
		logger:    settings.Logger,
		telemetry: telemetry,
		conn:      conn,
		closeChan: make(chan struct{}),
	}, nil
}

func (e *rulesExtension) Start(ctx context.Context, host component.Host) error {
	// fail open: with no rules loaded everything is written unreduced, which is always correct
	pollCtx, cancel := context.WithTimeout(ctx, rulesPollTimeout)
	e.pollReductionRules(pollCtx)
	cancel()
	e.startReductionRulesPoller()
	return nil
}

func (e *rulesExtension) Shutdown(ctx context.Context) error {
	close(e.closeChan)
	e.wg.Wait()
	e.telemetry.Shutdown()
	return e.conn.Close()
}

// Current returns the active rules, or nil.
func (e *rulesExtension) Current() metricreductionrule.RuleSet {
	rules := e.reductionRules.Load()
	if rules == nil {
		return nil
	}
	return *rules
}

// pollReductionRules refreshes the in-memory ruleset from ClickHouse. It fails
// open: on error the last known ruleset stays active, and an unreduced series is
// always correct, just at full fidelity.
func (e *rulesExtension) pollReductionRules(ctx context.Context) {
	rules, err := e.fetchReductionRules(ctx)
	if err != nil {
		e.logger.Error("failed to refresh metric reduction rules; keeping last known rules", zap.Error(err))
		e.telemetry.SignozmetricreductionrulesPollErrors.Add(ctx, 1)
		return
	}
	e.reductionRules.Store(&rules)
	e.telemetry.SignozmetricreductionrulesRuleCount.Record(ctx, int64(len(rules)))
}

func (e *rulesExtension) fetchReductionRules(ctx context.Context) (metricreductionrule.RuleSet, error) {
	// argMax over the distributed table picks the latest rule regardless of shard.
	query := fmt.Sprintf(`SELECT
			metric_name,
			argMax(labels, updated_at) AS labels,
			argMax(match_type, updated_at) AS match_type,
			argMax(effective_from_unix_milli, updated_at) AS effective_from_unix_milli,
			argMax(deleted, updated_at) AS deleted
		FROM %s.%s
		GROUP BY metric_name`, e.cfg.Database, e.cfg.RulesTable)

	rows, err := e.conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			e.logger.Warn("failed to close reduction rules result set", zap.Error(err))
		}
	}()

	rules := make(metricreductionrule.RuleSet)
	for rows.Next() {
		var (
			metricName    string
			labels        []string
			matchType     string
			effectiveFrom int64
			deleted       bool
		)
		if err := rows.Scan(&metricName, &labels, &matchType, &effectiveFrom, &deleted); err != nil {
			return nil, err
		}
		if deleted {
			continue
		}
		var keep bool
		switch matchType {
		case "", "drop":
			keep = false
		case "keep":
			keep = true
		default:
			// unknown mode: fail open (leave the metric unreduced) rather than
			// risk inverting drop/keep semantics on a typo.
			e.logger.Warn("ignoring reduction rule with unknown match_type",
				zap.String("metric_name", metricName), zap.String("match_type", matchType))
			continue
		}
		// drop mode with no labels is a no-op; keep mode with no labels is a
		// valid (maximal) reduction down to just the protected labels.
		if !keep && len(labels) == 0 {
			continue
		}
		keys := make(map[string]struct{}, len(labels))
		valid := true
		for _, label := range labels {
			if !keep {
				if _, protected := metricreductionrule.ProtectedLabels[label]; protected {
					e.logger.Warn("ignoring reduction rule that drops a protected label",
						zap.String("metric_name", metricName), zap.String("label", label))
					valid = false
					break
				}
			}
			keys[label] = struct{}{}
		}
		if !valid {
			continue
		}
		rules[metricName] = metricreductionrule.NewReductionRule(keys, keep, effectiveFrom)
	}
	return rules, rows.Err()
}

func (e *rulesExtension) startReductionRulesPoller() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.cfg.PollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-e.closeChan:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), rulesPollTimeout)
				e.pollReductionRules(ctx)
				cancel()
			}
		}
	}()
}
