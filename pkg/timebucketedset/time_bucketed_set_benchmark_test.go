package timebucketedset

import (
	"testing"
	"time"
)

func BenchmarkPlan_Hit(b *testing.B) {
	set, err := New(time.Hour, Config{MaxBuckets: 3, MaxBucketSize: 32 << 20})
	if err != nil {
		b.Fatal(err)
	}
	base := time.Date(2026, 9, 22, 4, 0, 0, 0, time.UTC).UnixMilli()
	id := []byte{1, 2, 3, 4, 5, 6, 7, 8, 0}
	set.Plan(id, base, base+1)
	set.Apply(SingleRow(id, base))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if cur, _ := set.Plan(id, base, base+1); cur {
			b.Fatal("registered id planned again")
		}
	}
}
