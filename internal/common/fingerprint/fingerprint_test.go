package fingerprint

import (
	"testing"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestFingerprint(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("key1", "value1")
	attrs.PutStr("key2", "value2")
	fp := NewFingerprint(ResourceFingerprintType, 0, attrs, nil)
	if fp.Hash() != 4672270062576455370 {
		t.Errorf("Fingerprint hash is incorrect: %d", fp.Hash())
	}
}

func TestFingerprintWithExtras(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("key1", "value1")
	attrs.PutStr("key2", "value2")
	fp := NewFingerprint(ResourceFingerprintType, 0, attrs, map[string]string{"key3": "value3"})
	if fp.Hash() != 5425952980149109402 {
		t.Errorf("Fingerprint hash is incorrect: %d", fp.Hash())
	}
}

func TestFingerprintSameWithDifferentOrder(t *testing.T) {
	for i := 0; i < 1000; i++ {
		attrs1 := pcommon.NewMap()
		attrs1.PutStr("key1", "value1")
		attrs1.PutStr("key2", "value2")

		attrs2 := pcommon.NewMap()
		attrs2.PutStr("key2", "value2")
		attrs2.PutStr("key1", "value1")

		fp1 := NewFingerprint(ResourceFingerprintType, 0, attrs1, nil)
		fp2 := NewFingerprint(ResourceFingerprintType, 0, attrs2, nil)
		if fp1.Hash() != fp2.Hash() {
			t.Errorf("Fingerprint hash is incorrect: %d", fp1.Hash())
		}
	}
}

func TestReducedNoDropEqualsOriginal(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("key1", "value1")
	attrs.PutStr("key2", "value2")
	fp := NewFingerprint(ResourceFingerprintType, InitialOffset, attrs, nil)
	reduced := fp.Reduced(InitialOffset, func(string) bool { return false })
	if fp.Hash() != reduced.Hash() {
		t.Errorf("reduced fingerprint with empty drop set should equal the original: %d != %d", fp.Hash(), reduced.Hash())
	}
}

func TestReducedCollapsesSeriesDifferingOnlyInDroppedKeys(t *testing.T) {
	drop := func(k string) bool { return k == "service.instance.id" }

	attrs1 := pcommon.NewMap()
	attrs1.PutStr("service.name", "app")
	attrs1.PutStr("service.instance.id", "instance-1")

	attrs2 := pcommon.NewMap()
	attrs2.PutStr("service.name", "app")
	attrs2.PutStr("service.instance.id", "instance-2")

	fp1 := NewFingerprint(ResourceFingerprintType, InitialOffset, attrs1, nil)
	fp2 := NewFingerprint(ResourceFingerprintType, InitialOffset, attrs2, nil)
	if fp1.Hash() == fp2.Hash() {
		t.Fatal("raw fingerprints should differ")
	}

	reduced1 := fp1.Reduced(InitialOffset, drop)
	reduced2 := fp2.Reduced(InitialOffset, drop)
	if reduced1.Hash() != reduced2.Hash() {
		t.Errorf("reduced fingerprints should collapse: %d != %d", reduced1.Hash(), reduced2.Hash())
	}
	if _, ok := reduced1.AttributesAsMap()["service.instance.id"]; ok {
		t.Error("dropped key should not be present in reduced attributes")
	}
}

func TestReducedChainCollapsesAcrossLevels(t *testing.T) {
	drop := func(k string) bool { return k == "service.instance.id" || k == "thread.id" }

	buildChain := func(instance, thread string) uint64 {
		resourceAttrs := pcommon.NewMap()
		resourceAttrs.PutStr("service.name", "app")
		resourceAttrs.PutStr("service.instance.id", instance)
		resourceFp := NewFingerprint(ResourceFingerprintType, InitialOffset, resourceAttrs, nil)

		scopeAttrs := pcommon.NewMap()
		scopeFp := NewFingerprint(ScopeFingerprintType, resourceFp.Hash(), scopeAttrs, map[string]string{"__scope.name__": "lib"})

		pointAttrs := pcommon.NewMap()
		pointAttrs.PutStr("thread.id", thread)
		pointAttrs.PutStr("status", "ok")
		pointFp := NewFingerprint(PointFingerprintType, scopeFp.Hash(), pointAttrs, map[string]string{"__temporality__": "Cumulative"})

		reducedResource := resourceFp.Reduced(InitialOffset, drop)
		reducedScope := scopeFp.Reduced(reducedResource.Hash(), drop)
		reducedPoint := pointFp.Reduced(reducedScope.Hash(), drop)
		return reducedPoint.HashWithName("http.server.requests.count")
	}

	a := buildChain("instance-1", "1")
	b := buildChain("instance-2", "7")
	if a != b {
		t.Errorf("reduced chains should collapse to one fingerprint: %d != %d", a, b)
	}

	c := buildChain("instance-1", "1")
	if a != c {
		t.Errorf("reduced chain must be deterministic: %d != %d", a, c)
	}
}

func TestReducedKeepsExtras(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("pod", "pod-1")
	fp := NewFingerprint(PointFingerprintType, InitialOffset, attrs, map[string]string{"le": "0.5", "__temporality__": "Cumulative"})
	reduced := fp.Reduced(InitialOffset, func(k string) bool { return k == "pod" })
	m := reduced.AttributesAsMap()
	if m["le"] != "0.5" || m["__temporality__"] != "Cumulative" {
		t.Errorf("extras should survive reduction: %v", m)
	}
	if _, ok := m["pod"]; ok {
		t.Error("dropped key should not survive reduction")
	}
}
