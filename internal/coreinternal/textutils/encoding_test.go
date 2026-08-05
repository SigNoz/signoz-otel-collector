// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package textutils

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

func TestDecode(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
		input    []byte
		expected string
	}{
		{
			name:     "UTF-8 simple",
			encoding: "utf-8",
			input:    []byte("hello world"),
			expected: "hello world",
		},
		{
			name:     "UTF-8 JSON",
			encoding: "utf-8",
			input:    []byte(`{"timestamp":"2026-01-01T00:00:00Z","level":"info","message":"test"}`),
			expected: `{"timestamp":"2026-01-01T00:00:00Z","level":"info","message":"test"}`,
		},
		{
			name:     "UTF-8 Korean",
			encoding: "utf-8",
			input:    []byte("안녕하세요"),
			expected: "안녕하세요",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encCfg := NewEncodingConfig()
			encCfg.Encoding = tt.encoding
			enc, err := encCfg.Build()
			require.NoError(t, err)

			result, err := enc.Decode(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestDecodeConcurrency(t *testing.T) {
	encCfg := NewEncodingConfig()
	encCfg.Encoding = "utf-8"
	enc, err := encCfg.Build()
	require.NoError(t, err)

	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	errors := make(chan error, goroutines*iterations)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			// each goroutine uses unique message
			msg := []byte(strings.Repeat("x", id+1) + "-test-message")
			expected := string(msg)

			for j := 0; j < iterations; j++ {
				result, err := enc.Decode(msg)
				if err != nil {
					errors <- err
					return
				}
				if string(result) != expected {
					t.Errorf("goroutine %d: expected %q, got %q", id, expected, string(result))
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("decode error: %v", err)
	}
}

func TestDecodeLargeMessage(t *testing.T) {
	encCfg := NewEncodingConfig()
	encCfg.Encoding = "utf-8"
	enc, err := encCfg.Build()
	require.NoError(t, err)

	// larger than defaultBufSize (4KB)
	largeMsg := []byte(strings.Repeat("a", 10*1024))

	result, err := enc.Decode(largeMsg)
	require.NoError(t, err)
	assert.Equal(t, string(largeMsg), string(result))
}

func TestDecodeResultIsolation(t *testing.T) {
	encCfg := NewEncodingConfig()
	encCfg.Encoding = "utf-8"
	enc, err := encCfg.Build()
	require.NoError(t, err)

	msg1 := []byte("message-one")
	msg2 := []byte("message-two")

	result1, err := enc.Decode(msg1)
	require.NoError(t, err)

	result2, err := enc.Decode(msg2)
	require.NoError(t, err)

	// verify results are isolated
	assert.Equal(t, "message-one", string(result1))
	assert.Equal(t, "message-two", string(result2))
}

func TestUTF8Encoding(t *testing.T) {
	tests := []struct {
		name         string
		encoding     encoding.Encoding
		encodingName string
	}{
		{
			name:         "UTF8 encoding",
			encoding:     unicode.UTF8,
			encodingName: "utf8",
		},
		{
			name:         "GBK encoding",
			encoding:     simplifiedchinese.GBK,
			encodingName: "gbk",
		},
		{
			name:         "SHIFT_JIS encoding",
			encoding:     japanese.ShiftJIS,
			encodingName: "shift_jis",
		},
		{
			name:         "EUC-KR encoding",
			encoding:     korean.EUCKR,
			encodingName: "euc-kr",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			encCfg := NewEncodingConfig()
			encCfg.Encoding = test.encodingName
			enc, err := encCfg.Build()
			assert.NoError(t, err)
			assert.Equal(t, test.encoding, enc.Encoding)
		})
	}
}
