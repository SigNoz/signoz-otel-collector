package signozclickhousemeter

type batch struct {
	samples []*sample
}

func newBatch() *batch {
	return &batch{
		samples: make([]*sample, 0),
	}
}

func (b *batch) addSample(sample *sample) {
	b.samples = append(b.samples, sample)
}
