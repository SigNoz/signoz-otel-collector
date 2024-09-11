package signozlogspipelineprocessor

import (
	"context"
	"fmt"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/entry"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

// A stanza operator that consumes stanza entries and converts them to pdata.Log
// for passing them on to the next otel consumer.
type stanzaToOtelConsumer struct {
	nextConsumer consumer.Logs

	logger *zap.Logger

	// One plog.Logs can contain many log records. While the otel processor ConsumeLogs
	// works with one plog.Logs at a time, the stanza pipeline works one log entry at a time.
	// `processedEntries` accumulates entries processed for a single plog.Logs until the
	// signozlogspipeline processor flushes these entries out - converting them back into
	// a single plog.Logs sent to `nextConsumer.ConsumeLogs`
	processedEntries []*entry.Entry

	lock sync.Mutex
}

// stanzaToOtelConsumer must implement the stanza operator.Operator interface
func (c *stanzaToOtelConsumer) ID() string {
	return "stanza-otel-consumer"
}

func (c *stanzaToOtelConsumer) Type() string {
	return "stanza-otel-consumer"
}

func (c *stanzaToOtelConsumer) Start(_ operator.Persister) error {
	return nil
}

func (c *stanzaToOtelConsumer) Stop() error {
	return nil
}

func (c *stanzaToOtelConsumer) CanOutput() bool {
	return false
}

func (c *stanzaToOtelConsumer) Outputs() []operator.Operator {
	return []operator.Operator{}
}

func (c *stanzaToOtelConsumer) GetOutputIDs() []string {
	return []string{}
}

func (c *stanzaToOtelConsumer) SetOutputs([]operator.Operator) error {
	return fmt.Errorf("outputs not supported")
}

func (c *stanzaToOtelConsumer) SetOutputIDs([]string) {
}

func (c *stanzaToOtelConsumer) CanProcess() bool {
	return true
}

// Process an entry passed on to this operator by a stanza pipeline
// This operator is expected to be used as a sink in the stanza pipeline
func (c *stanzaToOtelConsumer) Process(ctx context.Context, entry *entry.Entry) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.processedEntries = append(c.processedEntries, entry)
	return nil
}

// One plog.Logs can contain many log records. While the otel processor ConsumeLogs
// works with one plog.Logs at a time, the stanza pipeline works one log entry at a time.
// `processedEntries` accumulates entries processed for a single plog.Logs until the
// signozlogspipeline processor flushes these entries out - converting them back into
// a single plog.Logs sent to `nextConsumer.ConsumeLogs`
func (c *stanzaToOtelConsumer) flush(ctx context.Context) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	plogs := convertEntriesToPlogs(c.processedEntries)

	err := c.nextConsumer.ConsumeLogs(ctx, plogs)
	if err != nil {
		return err
	}

	c.processedEntries = []*entry.Entry{}

	return nil
}

func (c *stanzaToOtelConsumer) Logger() *zap.Logger {
	return c.logger
}
