package signozclickhousemetrics

import (
	"encoding/binary"
	"fmt"
	"strings"

	chproto "github.com/ClickHouse/ch-go/proto"
)

// Flags of ClickHouse's quantilesDD state serialization, which follows
// DataDog's sketches-go encoding.
const (
	ddSketchFlagIndexMappingBaseLogarithmic byte = 0b10
	ddSketchFlagPositiveStore               byte = 0b01
	ddSketchFlagNegativeStore               byte = 0b11
	ddSketchFlagZeroCount                   byte = 1 << 2
	ddSketchBinEncodingIndexDeltasAndCounts byte = 1 << 2
	ddSketchBinEncodingContiguousCounts     byte = 3 << 2
)

type ddSketch struct {
	gamma     float64
	positive  ddSketchStore
	negative  ddSketchStore
	zeroCount float64
}

type ddSketchStore struct {
	offset int32
	counts []float64
}

func (s ddSketch) encode(b *chproto.Buffer) {
	b.PutByte(ddSketchFlagIndexMappingBaseLogarithmic)
	b.PutFloat64(s.gamma)
	// index offset
	b.PutFloat64(0)
	b.PutByte(ddSketchFlagPositiveStore)
	s.positive.encode(b)
	b.PutByte(ddSketchFlagNegativeStore)
	s.negative.encode(b)
	b.PutByte(ddSketchFlagZeroCount)
	b.PutFloat64(s.zeroCount)
}

// encode writes an empty store as zero index-delta bins.
func (s ddSketchStore) encode(b *chproto.Buffer) {
	if len(s.counts) == 0 {
		b.PutByte(ddSketchBinEncodingIndexDeltasAndCounts)
		b.PutUVarInt(0)
		return
	}
	b.PutByte(ddSketchBinEncodingContiguousCounts)
	b.PutUVarInt(uint64(len(s.counts)))
	b.Buf = binary.AppendVarint(b.Buf, int64(s.offset))
	// index delta between successive bins
	b.Buf = binary.AppendVarint(b.Buf, 1)
	for _, count := range s.counts {
		b.PutFloat64(count)
	}
}

// ddSketchColumn takes its type from the server, so the quantile levels
// declared on the table need not be repeated here.
type ddSketchColumn struct {
	typ  chproto.ColumnType
	data []ddSketch
}

func (c *ddSketchColumn) Infer(t chproto.ColumnType) error {
	if !strings.HasPrefix(string(t), "AggregateFunction(quantilesDD(") {
		return fmt.Errorf("unsupported sketch column type %q", t)
	}
	c.typ = t
	return nil
}

func (c *ddSketchColumn) Type() chproto.ColumnType { return c.typ }

func (c *ddSketchColumn) Rows() int { return len(c.data) }

func (c *ddSketchColumn) Append(v ddSketch) { c.data = append(c.data, v) }

func (c *ddSketchColumn) Reset() { c.data = c.data[:0] }

func (c *ddSketchColumn) EncodeColumn(b *chproto.Buffer) {
	for _, v := range c.data {
		v.encode(b)
	}
}

func (c *ddSketchColumn) WriteColumn(w *chproto.Writer) {
	w.ChainBuffer(c.EncodeColumn)
}
