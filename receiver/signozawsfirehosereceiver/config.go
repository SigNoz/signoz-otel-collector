// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package signozawsfirehosereceiver // import "github.com/SigNoz/signoz-otel-collector/receiver/signozawsfirehosereceiver"

import (
	"errors"

	"go.opentelemetry.io/collector/config/confighttp"
)

type Config struct {
	// ServerConfig is used to set up the Firehose delivery
	// endpoint. The Firehose delivery stream expects an HTTPS
	// endpoint, so TLSSettings must be used to enable that.
	confighttp.ServerConfig `mapstructure:",squash"`
	// RecordType is the key used to determine which unmarshaler to use
	// when receiving the requests.
	RecordType string `mapstructure:"record_type"`
}

// Validate checks that the endpoint and record type exist and
// are valid.
func (c *Config) Validate() error {
	if c.Endpoint == "" {
		return errors.New("must specify endpoint")
	}
	// If a record type is specified, it must be valid.
	// An empty string is acceptable, however, because it will use a telemetry-type-specific default.
	if c.RecordType != "" {
		return validateRecordType(c.RecordType)
	}
	return nil
}
