package ottlfunctions

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

type HexToIntArguments[K any] struct {
	Target ottl.StandardStringGetter[K]
}

func NewHexToIntFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("HexToInt", &HexToIntArguments[K]{}, createHexToIntFunction[K])
}

func createHexToIntFunction[K any](_ ottl.FunctionContext, oArgs ottl.Arguments) (ottl.ExprFunc[K], error) {
	args, ok := oArgs.(*HexToIntArguments[K])

	if !ok {
		return nil, fmt.Errorf("HexToIntFactory args must be of type *HexToIntArguments[K]")
	}

	return hexToInt(args.Target)
}

func hexToInt[K any](target ottl.StandardStringGetter[K]) (ottl.ExprFunc[K], error) {
	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		matches, err := regexp.Match(`^(0x|0X)?[a-fA-F0-9]+$`, []byte(val))
		if err != nil {
			return nil, fmt.Errorf("could not test %s for being hex with regex: %w", val, err)
		}
		if !matches || len(val)%2 != 0 {
			return nil, fmt.Errorf("invalid hex value: %s", val)
		}

		val = strings.ToLower(val)
		if !strings.HasPrefix(val, "0x") {
			val = "0x" + val
		}

		return strconv.ParseInt(val, 0, 64)
	}, nil
}
