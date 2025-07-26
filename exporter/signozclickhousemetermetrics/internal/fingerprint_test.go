package internal

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
