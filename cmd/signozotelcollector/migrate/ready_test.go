package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCannonicalVersion(t *testing.T) {
	tests := []struct {
		Name            string
		CurrentVersion  string
		ExpectedVersion string
		Pass            bool
	}{
		{
			Name:            "empty strings",
			CurrentVersion:  "",
			ExpectedVersion: "",
			Pass:            true,
		},
		{
			Name:            "minimal version match",
			CurrentVersion:  "22.8.1",
			ExpectedVersion: "22.8.1",
			Pass:            true,
		},
		{
			Name:            "same major.minor.patch.extradata",
			CurrentVersion:  "22.8.1.1",
			ExpectedVersion: "22.8.1",
			Pass:            true,
		},
		{
			Name:            "different patch",
			CurrentVersion:  "22.8.2.4",
			ExpectedVersion: "22.8.1",
			Pass:            false,
		},
		{
			Name:            "expected version has extra fields",
			CurrentVersion:  "21.1.99.42",
			ExpectedVersion: "21.1.99.2",
			Pass:            true,
		},
		{
			Name:            "version less than 3 segments",
			CurrentVersion:  "21.2",
			ExpectedVersion: "21.2",
			Pass:            true,
		},
		{
			Name:            "different minor version",
			CurrentVersion:  "21.3.5",
			ExpectedVersion: "21.2.5",
			Pass:            false,
		},
	}

	for _, test := range tests {
		ready := ready{version: test.ExpectedVersion}
		canonicalCurrent := ready.getCanonicalVersion(test.CurrentVersion)
		canonicalExpected := ready.getCanonicalVersion(test.ExpectedVersion)

		pass := canonicalCurrent == canonicalExpected
		assert.Equal(t, test.Pass, pass)
	}
}
