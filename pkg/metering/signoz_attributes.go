package metering

import (
	"regexp"
	"strings"

	"github.com/SigNoz/signoz-otel-collector/constants"
)

var (
	ExcludeSigNozWorkspaceResourceAttrs = regexp.MustCompile("^signoz.workspace.*")
	// Costs the signozllmpricing processor attaches to spans; derived data, not billed.
	ExcludeSigNozLLMPricingSpanAttrs = regexp.MustCompile(`^signoz\.gen_ai\.`)
	ExcludeSigNozInternalAttrPrefix  = regexp.MustCompile("^" + regexp.QuoteMeta(constants.SignozInternalAttrPrefix))

	// Keys matching these are not billed, in resources or attributes.
	LogsExcludePatterns   = []*regexp.Regexp{ExcludeSigNozWorkspaceResourceAttrs, ExcludeSigNozInternalAttrPrefix}
	TracesExcludePatterns = []*regexp.Regexp{ExcludeSigNozWorkspaceResourceAttrs, ExcludeSigNozLLMPricingSpanAttrs}
)

func IsBillableLogAttribute(key string) bool {
	return isBillable(key, LogsExcludePatterns)
}

func IsBillableSpanAttribute(key string) bool {
	return isBillable(key, TracesExcludePatterns)
}

func isBillable(key string, patterns []*regexp.Regexp) bool {
	// Every exclude pattern starts with "signoz"; skip the regexes for all other keys.
	if !strings.HasPrefix(key, "signoz") {
		return true
	}
	for _, pattern := range patterns {
		if pattern.MatchString(key) {
			return false
		}
	}
	return true
}
