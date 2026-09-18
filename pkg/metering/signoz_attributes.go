package metering

import "regexp"

var (
	ExcludeSigNozWorkspaceResourceAttrs = regexp.MustCompile("^signoz.workspace.*")
	// Costs the signozllmpricing processor attaches to spans; derived data, not billed.
	ExcludeSigNozLLMPricingSpanAttrs = regexp.MustCompile(`^signoz\.gen_ai\.`)
)
