// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp

import (
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/config/configtls"

	"github.com/SigNoz/signoz-otel-collector/pkg/stanza/operator/operatortest"
)

func TestUnmarshal(t *testing.T) {
	operatortest.ConfigUnmarshalTests{
		DefaultConfig: NewConfig(),
		TestsFile:     filepath.Join(".", "testdata", "config.yaml"),
		Tests: []operatortest.ConfigUnmarshalTest{
			{
				Name:      "default",
				ExpectErr: false,
				Expect:    NewConfig(),
			},
			{
				Name:      "all",
				ExpectErr: false,
				Expect: func() *Config {
					cfg := NewConfig()
					cfg.MaxLogSize = 1000000
					cfg.ListenAddress = "10.0.0.1:9000"
					cfg.AddAttributes = true
					cfg.Encoding = "utf-8"
					cfg.SplitConfig.LineStartPattern = "ABC"
					cfg.TLS = &configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "foo",
							KeyFile:  "foo2",
							CAFile:   "foo3",
						},
						ClientCAFile: "foo4",
					}
					return cfg
				}(),
			},
		},
	}.Run(t)
}
