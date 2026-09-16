package tagdedup

import (
	"testing"

	"github.com/SigNoz/signoz-otel-collector/utils"
	"github.com/stretchr/testify/assert"
)

func TestKeyIDDistinguishesComponents(t *testing.T) {
	base := KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, false)

	testCases := []struct {
		name     string
		key      string
		tagType  utils.TagType
		dataType utils.FieldDataType
		isColumn bool
	}{
		{name: "Key", key: "http.method", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeFloat64, isColumn: false},
		{name: "TagType", key: "http.status_code", tagType: utils.TagTypeResource, dataType: utils.FieldDataTypeFloat64, isColumn: false},
		{name: "DataType", key: "http.status_code", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeString, isColumn: false},
		{name: "IsColumn", key: "http.status_code", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeFloat64, isColumn: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			id := KeyID(testCase.key, testCase.tagType, testCase.dataType, testCase.isColumn)
			assert.NotEqual(t, base, id)
		})
	}
}

func TestKeyIDIsDeterministic(t *testing.T) {
	a := KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, false)
	b := KeyID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, false)
	assert.Equal(t, a, b)
}

func TestKeyIDDoesNotCollideOnConcatenation(t *testing.T) {
	a := KeyID("ab", utils.TagType("c"), utils.FieldDataType("d"), false)
	b := KeyID("a", utils.TagType("bc"), utils.FieldDataType("d"), false)
	assert.NotEqual(t, a, b)
}

func TestValueIDDistinguishesComponents(t *testing.T) {
	base := ValueID("http.status_code", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200)

	testCases := []struct {
		name        string
		key         string
		tagType     utils.TagType
		dataType    utils.FieldDataType
		stringValue string
		numberValue float64
	}{
		{name: "Key", key: "http.method", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeFloat64, stringValue: "", numberValue: 200},
		{name: "TagType", key: "http.status_code", tagType: utils.TagTypeResource, dataType: utils.FieldDataTypeFloat64, stringValue: "", numberValue: 200},
		{name: "DataType", key: "http.status_code", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeString, stringValue: "", numberValue: 200},
		{name: "StringValue", key: "http.status_code", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeFloat64, stringValue: "OK", numberValue: 200},
		{name: "NumberValue", key: "http.status_code", tagType: utils.TagTypeAttribute, dataType: utils.FieldDataTypeFloat64, stringValue: "", numberValue: 404},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			id := ValueID(testCase.key, testCase.tagType, testCase.dataType, testCase.stringValue, testCase.numberValue)
			assert.NotEqual(t, base, id)
		})
	}
}

func TestValueIDFormatsNumbersConsistently(t *testing.T) {
	integer := ValueID("latency", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200)
	integerFloat := ValueID("latency", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200.0)
	assert.Equal(t, integer, integerFloat)

	fraction := ValueID("latency", utils.TagTypeAttribute, utils.FieldDataTypeFloat64, "", 200.5)
	assert.NotEqual(t, integer, fraction)
}

func TestDeduperSeenKeyID(t *testing.T) {
	d := New()
	id := KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, false)

	assert.False(t, d.SeenKeyID(id))
	assert.True(t, d.SeenKeyID(id))
	assert.False(t, d.SeenKeyID(KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, true)))
}

func TestDeduperSeenValueID(t *testing.T) {
	d := New()
	id := ValueID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, "GET", 0)

	assert.False(t, d.SeenValueID(id))
	assert.True(t, d.SeenValueID(id))
	assert.False(t, d.SeenValueID(ValueID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, "POST", 0)))
}

func TestDeduperKeyAndValueSpacesAreIndependent(t *testing.T) {
	d := New()
	id := KeyID("http.method", utils.TagTypeAttribute, utils.FieldDataTypeString, false)

	d.SeenKeyID(id)
	assert.False(t, d.SeenValueID(id))
}
