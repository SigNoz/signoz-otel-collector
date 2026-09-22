package timebucketedset

import "fmt"

type opType int

const (
	opPlan opType = iota
	opApply
)

// Step is one Plan or Apply call in a scenario. A nil expectation is not
// checked.
type Step struct {
	op                          opType
	id                          []byte
	bucketStartUnixMilliseconds int64
	nowUnixMilliseconds         int64
	expectedCurrent             *bool
	expectedNext                *bool
}

func PlanStep(id []byte, bucketStartUnixMilliseconds, nowUnixMilliseconds int64, expectedCurrent, expectedNext *bool) Step {
	return Step{
		op:                          opPlan,
		id:                          id,
		bucketStartUnixMilliseconds: bucketStartUnixMilliseconds,
		nowUnixMilliseconds:         nowUnixMilliseconds,
		expectedCurrent:             expectedCurrent,
		expectedNext:                expectedNext,
	}
}

func ApplyStep(id []byte, bucketStartUnixMilliseconds int64) Step {
	return Step{
		op:                          opApply,
		id:                          id,
		bucketStartUnixMilliseconds: bucketStartUnixMilliseconds,
	}
}

// RunSteps executes steps in order and stops at the first Plan whose answer differs from a non-nil expectation.
func RunSteps(set *Set, steps []Step) error {
	items := &Items{}

	for i, step := range steps {
		switch step.op {
		case opPlan:
			current, next := set.Plan(step.id, step.bucketStartUnixMilliseconds, step.nowUnixMilliseconds)
			if step.expectedCurrent != nil && current != *step.expectedCurrent {
				return fmt.Errorf("step %d: current is %t, expected %t", i, current, *step.expectedCurrent)
			}
			if step.expectedNext != nil && next != *step.expectedNext {
				return fmt.Errorf("step %d: next is %t, expected %t", i, next, *step.expectedNext)
			}

		case opApply:
			items.Add(step.id, step.bucketStartUnixMilliseconds)
			set.Apply(items)

		default:
			return fmt.Errorf("step %d: unknown op %d", i, step.op)
		}
	}

	return nil
}

func ExpectedBool(v bool) *bool {
	return &v
}
