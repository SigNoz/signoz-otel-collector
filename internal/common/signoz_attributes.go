package common

import "regexp"

var (
	ExcludeSigNozWorkspaceResourceAttrs = regexp.MustCompile("^signoz.workspace.*")
)
