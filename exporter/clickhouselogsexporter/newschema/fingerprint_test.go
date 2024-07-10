package newschema

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateFingerprint(t *testing.T) {
	testCases := []struct {
		Name          string
		ResourceAttrs map[string]any
		FingerPrint   string
	}{
		{
			Name:          "Random resource attr",
			ResourceAttrs: map[string]any{"a": "b"},
			FingerPrint:   "hash=15182603570120227210",
		},
		{
			Name:          "Few attrs from the hierarchy",
			ResourceAttrs: map[string]any{"ec2.tag.env": "fn-prod", "host.image.id": "ami-fce3c696"},
			FingerPrint:   "ec2.tag.env=fn-prod;hash=5580615729524003981",
		},
		{
			Name:          "More than one attrs from the hierarchy",
			ResourceAttrs: map[string]any{"cloudwatch.log.stream": "mystr", "ec2.tag.env": "fn-prod", "host.image.id": "ami-fce3c696"},
			FingerPrint:   "ec2.tag.env=fn-prod;cloudwatch.log.stream=mystr;hash=10649409385811604510",
		},
	}

	for _, ts := range testCases {
		res := CalculateFingerprint(ts.ResourceAttrs, ResourceHierarchy())
		assert.Equal(t, ts.FingerPrint, res)
	}
}
