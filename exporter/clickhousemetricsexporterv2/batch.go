package clickhousemetricsexporterv2

type batch struct {
	samples  []*sample
	expHist  []*exponentialHistogramSample
	ts       []*ts
	metadata []*metadata

	metaSeen map[string]struct{}
}
