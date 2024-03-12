package ottlfunctions

import (
	"context"
	"fmt"
	"reflect"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

type BytesGetter[K any] struct {
	Getter func(ctx context.Context, tCtx K) (any, error)
}

func (g BytesGetter[K]) Get(ctx context.Context, tCtx K) ([]byte, error) {
	val, err := g.Getter(ctx, tCtx)
	if err != nil {
		return nil, fmt.Errorf("error getting value in %T: %w", g, err)
	}
	if val == nil {
		return nil, nil
	}
	switch v := val.(type) {
	case []byte:
		return v, nil
	case pcommon.ByteSlice:
		return v.AsRaw(), nil
	default:
		vType := reflect.TypeOf(v)
		if vType.Kind() == reflect.Array && vType.Elem().Kind() == reflect.Uint8 {
			result := make([]byte, vType.Len())
			vValue := reflect.ValueOf(v)
			for i := 0; i < vType.Len(); i++ {
				iv := vValue.Index(i)
				ivByte := uint8(iv.Uint())
				result[i] = ivByte
			}
			return result, nil
		}
		return nil, ottl.TypeError(fmt.Sprintf("unsupported type: %T", v))
	}
}

type BytesToStringArguments[K any] struct {
	Target BytesGetter[K]
}

func NewBytesToStringFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("BytesToString", &BytesToStringArguments[K]{}, createBytesToStringFunction[K])
}

func createBytesToStringFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*BytesToStringArguments[K])

	if !ok {
		return nil, fmt.Errorf("BytesToStringFactory args must be of type *BytesToStringArguments[K]")
	}

	return bytesToString(args.Target)
}

func bytesToString[K any](target BytesGetter[K]) (ottl.ExprFunc[K], error) {

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}
		return string(val), nil
	}, nil
}
