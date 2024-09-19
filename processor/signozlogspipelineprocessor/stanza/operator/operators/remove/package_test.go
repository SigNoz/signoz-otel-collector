// Brought in as is from opentelemetry-collector-contrib

package remove

import (
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
