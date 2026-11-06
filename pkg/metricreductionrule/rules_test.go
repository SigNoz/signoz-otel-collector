package metricreductionrule

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
)

func TestReductionRule_AppliesAt(t *testing.T) {
	testCases := []struct {
		name      string
		unixMilli int64
		expected  bool
	}{
		{name: "AtEffectiveFrom", unixMilli: 1700000000000, expected: true},
		{name: "OneMilliBefore", unixMilli: 1699999999999, expected: false},
		{name: "OneMilliAfter", unixMilli: 1700000000001, expected: true},
	}

	rule := NewReductionRule(map[string]struct{}{"pod": {}}, false, 1700000000000)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expected, rule.AppliesAt(testCase.unixMilli))
		})
	}
}

func TestReductionRule_Drop(t *testing.T) {
	testCases := []struct {
		name     string
		keep     bool
		labels   []string
		key      string
		expected bool
	}{
		{name: "Drop_Listed", keep: false, labels: []string{"pod"}, key: "pod", expected: true},
		{name: "Drop_Unlisted", keep: false, labels: []string{"pod"}, key: "host.name", expected: false},
		{name: "Drop_UnlistedProtected", keep: false, labels: []string{"pod"}, key: "le", expected: false},
		{name: "Keep_Listed", keep: true, labels: []string{"service.name"}, key: "service.name", expected: false},
		{name: "Keep_Unlisted", keep: true, labels: []string{"service.name"}, key: "pod", expected: true},
		{name: "Keep_UnlistedLe", keep: true, labels: []string{"service.name"}, key: "le", expected: false},
		{name: "Keep_UnlistedQuantile", keep: true, labels: nil, key: "quantile", expected: false},
		{name: "Keep_UnlistedName", keep: true, labels: nil, key: "__name__", expected: false},
		{name: "Keep_UnlistedTemporality", keep: true, labels: nil, key: "__temporality__", expected: false},
		{name: "Keep_UnlistedDeploymentEnvironment", keep: true, labels: nil, key: "deployment.environment", expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			keys := make(map[string]struct{}, len(testCase.labels))
			for _, label := range testCase.labels {
				keys[label] = struct{}{}
			}
			rule := NewReductionRule(keys, testCase.keep, 0)
			assert.Equal(t, testCase.expected, rule.Drop(testCase.key))
		})
	}
}

// host serves these extensions; componenttest.NewNopHost has none.
type host map[component.ID]component.Component

func (h host) GetExtensions() map[component.ID]component.Component {
	return h
}

// source is an extension that implements Source.
type source struct {
	component.StartFunc
	component.ShutdownFunc
}

func (source) Current() RuleSet {
	return nil
}

func TestFromHost(t *testing.T) {
	id := component.MustNewID("signozmetricreductionrules")
	rulesExtension := source{}
	notSource := struct {
		component.StartFunc
		component.ShutdownFunc
	}{}

	testCases := []struct {
		name       string
		extensions map[component.ID]component.Component
		expected   Source
		err        string
	}{
		{name: "ExtensionPresent_Found", extensions: map[component.ID]component.Component{id: rulesExtension}, expected: rulesExtension},
		{name: "ExtensionMissing_Error", extensions: map[component.ID]component.Component{component.MustNewID("file_storage"): rulesExtension}, err: `metric reduction rules extension "signozmetricreductionrules" not found`},
		{name: "ExtensionNotSource_Error", extensions: map[component.ID]component.Component{id: notSource}, err: `extension "signozmetricreductionrules" is not a metric reduction rules extension`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := FromHost(host(testCase.extensions), id)
			if testCase.err != "" {
				assert.EqualError(t, err, testCase.err)
				assert.Nil(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, got)
		})
	}
}
