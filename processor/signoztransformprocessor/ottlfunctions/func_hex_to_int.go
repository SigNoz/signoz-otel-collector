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
	hexRegex := regexp.MustCompile(`^(0x)?[a-f0-9]+$`)

	return func(ctx context.Context, tCtx K) (interface{}, error) {
		val, err := target.Get(ctx, tCtx)
		if err != nil {
			return nil, err
		}

		val = strings.ToLower(val)

		isValidHex := hexRegex.Match([]byte(val))
		if err != nil {
			return nil, fmt.Errorf("could not test %s for being hex with regex: %w", val, err)
		}
		if !isValidHex {
			return nil, fmt.Errorf("invalid hex value: %s", val)
		}

		val = strings.TrimPrefix(val, "0x")

		return strconv.ParseInt(val, 16, 64)
	}, nil
}
