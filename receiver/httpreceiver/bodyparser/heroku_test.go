package bodyparser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOctectCountingSplitter(t *testing.T) {
	t.Parallel()

	// cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	// require.NoError(t, err)

	tests := []struct {
		name     string
		PayLoad  string
		LogLines []string
		isError  bool
	}{
		{
			name:    "Test 1",
			PayLoad: `9 <1>1 - -`,
			LogLines: []string{
				`<1>1 - -`,
			},
		},
		{
			name:    "Test 2",
			PayLoad: `9 <1>1 - -9 <2>2 - -`,
			LogLines: []string{
				`<1>1 - -`,
				`<2>2 - -`,
			},
		},
		{
			name: "Test 3 with newline",
			PayLoad: `9 <1>1 - -
11 <2>2 - - s`,
			LogLines: []string{
				`<1>1 - -`,
				`<2>2 - - s`,
			},
		},
		{
			name: "Test 4 with newline and tabs",
			PayLoad: `9 <1>1 - -
			9 <2>1 - -
			9 <3>1 - -`,
			LogLines: []string{
				`<1>1 - -`,
				`<2>1 - -`,
				`<3>1 - -`,
			},
		},
		{
			name:    "Test 1",
			PayLoad: `250 <190>1 2023-10-13T10:48:11.04591+00:00 host app web.1 - 10.1.23.40 - - [13/Oct/2023:10:48:11 +0000] "GET / HTTP/1.1" 200 7450 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36"`,
			LogLines: []string{
				`<190>1 2023-10-13T10:48:11.04591+00:00 host app web.1 - 10.1.23.40 - - [13/Oct/2023:10:48:11 +0000] "GET / HTTP/1.1" 200 7450 "-" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/116.0.0.0 Safari/537.36"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := octectCountingSplitter(tt.PayLoad)
			assert.Equal(t, tt.LogLines, res)
		})
	}
}
