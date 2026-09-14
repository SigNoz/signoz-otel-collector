package tagdedup

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/utils"
)

func TestKeyIDDistinct(t *testing.T) {
	base := KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, false)

	ids := map[string]string{
		"base":        base,
		"key":         KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, false),
		"tagType":     KeyID("http.status_code", utils.TagTypeResource, utils.FieldDataTypeFloat64, false),
		"dataType":    KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeString, false),
		"isColumn":    KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, true),
		"determinism": KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, false),
	}

	seen := map[string]string{}
	for name, id := range ids {
		if name == "determinism" {
			if id != base {
				t.Fatalf("KeyID is not deterministic: %q != %q", id, base)
			}
			continue
		}
		if prev, ok := seen[id]; ok {
			t.Fatalf("KeyID collision between %q and %q: %q", prev, name, id)
		}
		seen[id] = name
	}
}

func TestKeyIDNoCollisionFromConcatenation(t *testing.T) {
	// Components containing the separator-adjacent characters must not collide.
	a := KeyID("ab", utils.TagType("c"), utils.FieldDataType("d"), false)
	b := KeyID("a", utils.TagType("bc"), utils.FieldDataType("d"), false)
	if a == b {
		t.Fatalf("expected distinct IDs, got %q", a)
	}
}

func TestValueIDDistinct(t *testing.T) {
	base := ValueID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200)

	ids := map[string]string{
		"base":        base,
		"key":         ValueID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200),
		"tagType":     ValueID("http.status_code", utils.TagTypeResource, utils.FieldDataTypeFloat64, "", 200),
		"dataType":    ValueID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeString, "", 200),
		"stringValue": ValueID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "OK", 200),
		"numberValue": ValueID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 404),
		"determinism": ValueID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200),
	}

	seen := map[string]string{}
	for name, id := range ids {
		if name == "determinism" {
			if id != base {
				t.Fatalf("ValueID is not deterministic: %q != %q", id, base)
			}
			continue
		}
		if prev, ok := seen[id]; ok {
			t.Fatalf("ValueID collision between %q and %q: %q", prev, name, id)
		}
		seen[id] = name
	}
}

func TestValueIDNumberFormatting(t *testing.T) {
	// Integer-valued floats must not have a trailing ".0" so that values
	// written by different code paths compare equal.
	a := ValueID("k", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200)
	b := ValueID("k", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200.0)
	if a != b {
		t.Fatalf("expected equal IDs, got %q and %q", a, b)
	}
	c := ValueID("k", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200.5)
	if a == c {
		t.Fatalf("expected distinct IDs for 200 and 200.5, got %q", a)
	}
}

func TestDeduperSeenKeyID(t *testing.T) {
	d := New()
	id := KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, false)

	if d.SeenKeyID(id) {
		t.Fatal("first SeenKeyID call should return false")
	}
	if !d.SeenKeyID(id) {
		t.Fatal("second SeenKeyID call should return true")
	}

	other := KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, true)
	if d.SeenKeyID(other) {
		t.Fatal("SeenKeyID for a different ID should return false")
	}
}

func TestDeduperSeenValueID(t *testing.T) {
	d := New()
	id := ValueID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, "GET", 0)

	if d.SeenValueID(id) {
		t.Fatal("first SeenValueID call should return false")
	}
	if !d.SeenValueID(id) {
		t.Fatal("second SeenValueID call should return true")
	}

	other := ValueID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, "POST", 0)
	if d.SeenValueID(other) {
		t.Fatal("SeenValueID for a different ID should return false")
	}
}

func TestDeduperKeysAndValuesAreIndependent(t *testing.T) {
	d := New()
	id := KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, false)

	d.SeenKeyID(id)
	if d.SeenValueID(id) {
		t.Fatal("key and value dedup spaces must be independent")
	}
}
