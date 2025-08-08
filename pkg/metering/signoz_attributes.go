package metering

import "regexp"

var (
	ExcludeSigNozWorkspaceResourceAttrs = regexp.MustCompile("^signoz.workspace.*")
)
