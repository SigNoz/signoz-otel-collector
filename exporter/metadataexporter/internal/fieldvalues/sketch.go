package fieldvalues

import (
	"math"
	"math/bits"
)

const sketchExactValues = 32

const sketchRegisters = 64

// sketch estimates the distinct values of one field on one resource. It counts
// exactly up to 32 values, and then with 64 registers (about 13% error). Coarse
// sets only need to find the field that multiplies the sets, so a rough count
// of a large field is enough and costs 64 bytes.
type sketch struct {
	exact     []uint64
	registers *[sketchRegisters]uint8
}

func (s *sketch) add(valueHash uint64) {
	if s.registers == nil {
		for _, v := range s.exact {
			if v == valueHash {
				return
			}
		}
		if len(s.exact) < sketchExactValues {
			s.exact = append(s.exact, valueHash)
			return
		}
		s.registers = &[sketchRegisters]uint8{}
		for _, v := range s.exact {
			s.addRegister(v)
		}
		s.exact = nil
	}
	s.addRegister(valueHash)
}

func (s *sketch) addRegister(valueHash uint64) {
	idx := valueHash >> 58
	rank := uint8(bits.LeadingZeros64(valueHash<<6|1<<5)) + 1
	if rank > s.registers[idx] {
		s.registers[idx] = rank
	}
}

func (s *sketch) estimate() float64 {
	if s.registers == nil {
		return float64(len(s.exact))
	}
	const m = float64(sketchRegisters)
	sum := 0.0
	zeros := 0
	for _, r := range s.registers {
		sum += math.Pow(2, -float64(r))
		if r == 0 {
			zeros++
		}
	}
	e := 0.709 * m * m / sum
	if e <= 2.5*m && zeros > 0 {
		e = m * math.Log(m/float64(zeros))
	}
	return math.Max(e, sketchExactValues)
}
