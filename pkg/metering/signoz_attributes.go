package metering

import (
	"regexp"

	"github.com/SigNoz/signoz-otel-collector/constants"
)

var (
	ExcludeSigNozWorkspaceResourceAttrs = regexp.MustCompile("^signoz.workspace.*")
	// Costs the signozllmpricing processor attaches to spans; derived data, not billed.
	ExcludeSigNozLLMPricingSpanAttrs = regexp.MustCompile(`^signoz\.gen_ai\.`)
	ExcludeSigNozInternalAttrPrefix  = regexp.MustCompile("^" + regexp.QuoteMeta(constants.SignozInternalAttrPrefix))
)
