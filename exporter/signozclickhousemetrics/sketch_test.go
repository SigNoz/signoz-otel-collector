package signozclickhousemetrics

import (
	"encoding/hex"
	"math"
	"testing"

	chproto "github.com/ClickHouse/ch-go/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDDSketchEncode(t *testing.T) {
	testCases := []struct {
		name   string
		sketch ddSketch
		want   string
	}{
		{
			name: "ContiguousStores_NegativeOffset",
			sketch: ddSketch{
				gamma:     math.Pow(2, math.Pow(2, -2)),
				positive:  ddSketchStore{offset: 1, counts: []float64{0, 3, 1}},
				negative:  ddSketchStore{offset: -2, counts: []float64{2}},
				zeroCount: 4,
			},
			want: "0215b7310afe06f33f0000000000000000010c03020200000000000000000000000000000840000000000000f03f030c0103020000000000000040040000000000001040",
		},
		{
			name:   "EmptyStores",
			sketch: ddSketch{gamma: 2},
			want:   "0200000000000000400000000000000000010400030400040000000000000000",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var b chproto.Buffer
			testCase.sketch.encode(&b)
			assert.Equal(t, testCase.want, hex.EncodeToString(b.Buf))
		})
	}
}

func TestDDSketchColumnInfer(t *testing.T) {
	testCases := []struct {
		name    string
		typ     chproto.ColumnType
		wantErr bool
	}{
		{name: "QuantilesDD", typ: "AggregateFunction(quantilesDD(0.01, 0.5, 0.75, 0.9, 0.95, 0.99), UInt64)"},
		{name: "OtherAggregateFunction_Error", typ: "AggregateFunction(uniq, UInt64)", wantErr: true},
		{name: "String_Error", typ: "String", wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var column ddSketchColumn
			err := column.Infer(testCase.typ)
			if testCase.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.typ, column.Type())
		})
	}
}
