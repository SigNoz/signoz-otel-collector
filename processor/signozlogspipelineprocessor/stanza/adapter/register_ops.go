// Register copies of stanza operators dedicated to signoz logs pipelines
package signozlogspipelinestanzaadapter

import (
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/add"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/copy"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/json"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/move"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/regex"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/remove"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/severity"
	_ "github.com/SigNoz/signoz-otel-collector/processor/signozlogspipelineprocessor/stanza/operator/operators/time"
)
