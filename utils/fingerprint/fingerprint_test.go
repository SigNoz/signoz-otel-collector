package fingerprint

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			Name:          "Vector and gcp",
			ResourceAttrs: map[string]any{"gcp.project": "myproject", "source_type": "gcp", "random_key": "val"},
			FingerPrint:   "gcp.project=myproject;source_type=gcp;hash=11162778839006855273",
		},
		{
			Name:          "service, env and component",
			ResourceAttrs: map[string]any{"service.name": "service", "env": "prod", "component": "service-component"},
			FingerPrint:   "service.name=service;env=prod;component=service-component;hash=18170521368096690780",
		},
	}

	for _, ts := range testCases {
		res := CalculateFingerprint(ts.ResourceAttrs, ResourceHierarchy())
		assert.Equal(t, ts.FingerPrint, res)
	}
}

func TestFindAttributeSynonyms(t *testing.T) {
	testHierarchy := &DimensionHierarchyNode{
		labels: []string{"1.a", "1.b"},

		subHierachies: []DimensionHierarchyNode{{
			labels: []string{"1.1.a", "1.1.b"},

			subHierachies: []DimensionHierarchyNode{{
				labels: []string{"1.1.1.a", "1.1.1.b"},
			}},
		}, {
			labels: []string{"1.2.a", "1.2.b"},
		}},
	}

	for _, tc := range []struct {
		Attribute        string
		ExpectedSynonyms []string
	}{
		{
			Attribute:        "non-existent-attribute",
			ExpectedSynonyms: nil,
		}, {
			Attribute:        "1.b",
			ExpectedSynonyms: []string{"1.a", "1.b"},
		}, {
			Attribute:        "1.2.a",
			ExpectedSynonyms: []string{"1.2.a", "1.2.b"},
		}, {
			Attribute:        "1.1.1.b",
			ExpectedSynonyms: []string{"1.1.1.a", "1.1.1.b"},
		},
	} {

		synonyms := testHierarchy.Synonyms(tc.Attribute)
		require.Equal(t, tc.ExpectedSynonyms, synonyms)
	}

}
