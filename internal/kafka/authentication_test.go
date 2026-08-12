// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
)

func TestAuthentication(t *testing.T) {
	saramaPlaintext := &sarama.Config{}
	saramaPlaintext.Net.SASL.Enable = true
	saramaPlaintext.Net.SASL.User = "jdoe"
	saramaPlaintext.Net.SASL.Password = "pass"

	saramaSASLSCRAM256Config := &sarama.Config{}
	saramaSASLSCRAM256Config.Net.SASL.Enable = true
	saramaSASLSCRAM256Config.Net.SASL.User = "jdoe"
	saramaSASLSCRAM256Config.Net.SASL.Password = "pass"
	saramaSASLSCRAM256Config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256

	saramaSASLSCRAM512Config := &sarama.Config{}
	saramaSASLSCRAM512Config.Net.SASL.Enable = true
	saramaSASLSCRAM512Config.Net.SASL.User = "jdoe"
	saramaSASLSCRAM512Config.Net.SASL.Password = "pass"
	saramaSASLSCRAM512Config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512

	saramaSASLHandshakeV1Config := &sarama.Config{}
	saramaSASLHandshakeV1Config.Net.SASL.Enable = true
	saramaSASLHandshakeV1Config.Net.SASL.User = "jdoe"
	saramaSASLHandshakeV1Config.Net.SASL.Password = "pass"
	saramaSASLHandshakeV1Config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
	saramaSASLHandshakeV1Config.Net.SASL.Version = sarama.SASLHandshakeV1

	saramaSASLPLAINConfig := &sarama.Config{}
	saramaSASLPLAINConfig.Net.SASL.Enable = true
	saramaSASLPLAINConfig.Net.SASL.User = "jdoe"
	saramaSASLPLAINConfig.Net.SASL.Password = "pass"

	saramaSASLPLAINConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext

	saramaTLSCfg := &sarama.Config{}
	saramaTLSCfg.Net.TLS.Enable = true
	tlsClient := configtls.ClientConfig{}
	tlscfg, err := tlsClient.LoadTLSConfig(context.Background())
	require.NoError(t, err)
	saramaTLSCfg.Net.TLS.Config = tlscfg

	saramaKerberosCfg := &sarama.Config{}
	saramaKerberosCfg.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaKerberosCfg.Net.SASL.Enable = true
	saramaKerberosCfg.Net.SASL.GSSAPI.ServiceName = "foobar"
	saramaKerberosCfg.Net.SASL.GSSAPI.AuthType = sarama.KRB5_USER_AUTH

	saramaKerberosKeyTabCfg := &sarama.Config{}
	saramaKerberosKeyTabCfg.Net.SASL.Mechanism = sarama.SASLTypeGSSAPI
	saramaKerberosKeyTabCfg.Net.SASL.Enable = true
	saramaKerberosKeyTabCfg.Net.SASL.GSSAPI.KeyTabPath = "/path"
	saramaKerberosKeyTabCfg.Net.SASL.GSSAPI.AuthType = sarama.KRB5_KEYTAB_AUTH

	tests := []struct {
		auth         Authentication
		saramaConfig *sarama.Config
		err          string
	}{
		{
			auth:         Authentication{PlainText: &PlainTextConfig{Username: "jdoe", Password: "pass"}},
			saramaConfig: saramaPlaintext,
		},
		{
			auth:         Authentication{TLS: &configtls.ClientConfig{}},
			saramaConfig: saramaTLSCfg,
		},
		{
			auth: Authentication{TLS: &configtls.ClientConfig{
				Config: configtls.Config{CAFile: "/doesnotexists"},
			}},
			saramaConfig: saramaTLSCfg,
			err:          "failed to load TLS config",
		},
		{
			auth:         Authentication{Kerberos: &KerberosConfig{ServiceName: "foobar"}},
			saramaConfig: saramaKerberosCfg,
		},
		{
			auth:         Authentication{Kerberos: &KerberosConfig{UseKeyTab: true, KeyTabPath: "/path"}},
			saramaConfig: saramaKerberosKeyTabCfg,
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-256"}},
			saramaConfig: saramaSASLSCRAM256Config,
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512"}},
			saramaConfig: saramaSASLSCRAM512Config,
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512", Version: 1}},
			saramaConfig: saramaSASLHandshakeV1Config,
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "PLAIN"}},
			saramaConfig: saramaSASLPLAINConfig,
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-222"}},
			saramaConfig: saramaSASLSCRAM512Config,
			err:          "invalid SASL Mechanism",
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "", Password: "pass", Mechanism: "SCRAM-SHA-512"}},
			saramaConfig: saramaSASLSCRAM512Config,
			err:          "username have to be provided",
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "", Mechanism: "SCRAM-SHA-512"}},
			saramaConfig: saramaSASLSCRAM512Config,
			err:          "password have to be provided",
		},
		{
			auth:         Authentication{SASL: &SASLConfig{Username: "jdoe", Password: "pass", Mechanism: "SCRAM-SHA-512", Version: 2}},
			saramaConfig: saramaSASLSCRAM512Config,
			err:          "invalid SASL Protocol Version",
		},
	}
	for _, test := range tests {
		t.Run("", func(t *testing.T) {
			config := &sarama.Config{}
			err := ConfigureAuthentication(test.auth, config)
			if test.err != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.err)
			} else {
				// equalizes SCRAMClientGeneratorFunc to do assertion with the same reference.
				config.Net.SASL.SCRAMClientGeneratorFunc = test.saramaConfig.Net.SASL.SCRAMClientGeneratorFunc
				// Normalize TLS CurvePreferences to avoid non-deterministic ordering issues
				if config.Net.TLS.Config != nil && test.saramaConfig.Net.TLS.Config != nil {
					sort.Slice(config.Net.TLS.Config.CurvePreferences, func(i, j int) bool {
						return config.Net.TLS.Config.CurvePreferences[i] < config.Net.TLS.Config.CurvePreferences[j]
					})
					sort.Slice(test.saramaConfig.Net.TLS.Config.CurvePreferences, func(i, j int) bool {
						return test.saramaConfig.Net.TLS.Config.CurvePreferences[i] < test.saramaConfig.Net.TLS.Config.CurvePreferences[j]
					})
				}
				assert.Equal(t, test.saramaConfig, config)
			}
		})
	}
}

func TestTLSCertificateReload(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")

	// Generate and write the initial certificate.
	cert1 := generateSelfSignedCert(t, "initial")
	writeCertAndKey(t, certFile, keyFile, cert1)

	tlsConfig := configtls.ClientConfig{
		Config: configtls.Config{
			CertFile:       certFile,
			KeyFile:        keyFile,
			ReloadInterval: 100 * time.Millisecond,
		},
		InsecureSkipVerify: true,
	}

	saramaCfg := &sarama.Config{}
	err := configureTLS(tlsConfig, saramaCfg)
	require.NoError(t, err)
	require.NotNil(t, saramaCfg.Net.TLS.Config)

	// Verify the initial certificate is served.
	getCert := saramaCfg.Net.TLS.Config.GetClientCertificate
	require.NotNil(t, getCert, "GetClientCertificate should be set when ReloadInterval is configured")

	initial, err := getCert(&tls.CertificateRequestInfo{})
	require.NoError(t, err)
	initialLeaf, err := x509.ParseCertificate(initial.Certificate[0])
	require.NoError(t, err)
	assert.Equal(t, "initial", initialLeaf.Subject.CommonName)

	// Replace cert files with a new certificate.
	cert2 := generateSelfSignedCert(t, "reloaded")
	writeCertAndKey(t, certFile, keyFile, cert2)

	// Wait for the reload interval to trigger.
	assert.Eventually(t, func() bool {
		reloaded, err := getCert(&tls.CertificateRequestInfo{})
		if err != nil {
			return false
		}
		leaf, err := x509.ParseCertificate(reloaded.Certificate[0])
		if err != nil {
			return false
		}
		return leaf.Subject.CommonName == "reloaded"
	}, 2*time.Second, 50*time.Millisecond, "certificate should be reloaded from disk")
}

type selfSignedCert struct {
	certPEM []byte
	keyPEM  []byte
}

func generateSelfSignedCert(t *testing.T, cn string) selfSignedCert {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return selfSignedCert{certPEM: certPEM, keyPEM: keyPEM}
}

func writeCertAndKey(t *testing.T, certFile, keyFile string, cert selfSignedCert) {
	t.Helper()
	require.NoError(t, os.WriteFile(certFile, cert.certPEM, 0600))
	require.NoError(t, os.WriteFile(keyFile, cert.keyPEM, 0600))
}
