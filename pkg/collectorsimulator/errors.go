package collectorsimulator

import (
	"errors"
	"fmt"
)

var ErrInvalidConfig = errors.New("invalid config")

func InvalidConfigError(err error) error {
	return fmt.Errorf("%w: %w", ErrInvalidConfig, err)
}
