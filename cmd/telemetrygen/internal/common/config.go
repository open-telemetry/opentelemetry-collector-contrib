// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/attribute"
)

var (
	errFormatOTLPAttributes       = fmt.Errorf("value should be of the format key=\"value\"")
	errDoubleQuotesOTLPAttributes = fmt.Errorf("value should be a string wrapped in double quotes")
)

const (
	defaultGRPCEndpoint = "localhost:4317"
	defaultHTTPEndpoint = "localhost:4318"
)

type KeyValue map[string]any

var _ pflag.Value = (*KeyValue)(nil)

func (v *KeyValue) String() string {
	return ""
}

func (v *KeyValue) Set(s string) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return errFormatOTLPAttributes
	}
	val := kv[1]
	if val == "true" {
		(*v)[kv[0]] = true
		return nil
	}
	if val == "false" {
		(*v)[kv[0]] = false
		return nil
	}
	if len(val) < 2 || !strings.HasPrefix(val, "\"") || !strings.HasSuffix(val, "\"") {
		return errDoubleQuotesOTLPAttributes
	}

	(*v)[kv[0]] = val[1 : len(val)-1]
	return nil
}

func (v *KeyValue) Type() string {
	return "map[string]any"
}

type Config struct {
	WorkerCount           int
	Rate                  float64
	TotalDuration         time.Duration
	ReportingInterval     time.Duration
	SkipSettingGRPCLogger bool

	// OTLP config
	CustomEndpoint      string
	Insecure            bool
	InsecureSkipVerify  bool
	UseHTTP             bool
	HTTPPath            string
	Headers             KeyValue
	ResourceAttributes  KeyValue
	TelemetryAttributes KeyValue

	// OTLP TLS configuration
	CaFile string

	// OTLP mTLS configuration
	ClientAuth ClientAuth
}

type ClientAuth struct {
	Enabled        bool
	ClientCertFile string
	ClientKeyFile  string
}

// Endpoint returns the appropriate endpoint URL based on the selected communication mode (gRPC or HTTP)
// or custom endpoint provided in the configuration.
func (c *Config) Endpoint() string {
	if c.CustomEndpoint != "" {
		return c.CustomEndpoint
	}
	if c.UseHTTP {
		return defaultHTTPEndpoint
	}
	return defaultGRPCEndpoint
}

func (c *Config) GetAttributes() []attribute.KeyValue {
	var attributes []attribute.KeyValue

	if len(c.ResourceAttributes) > 0 {
		for k, t := range c.ResourceAttributes {
			switch v := t.(type) {
			case string:
				attributes = append(attributes, attribute.String(k, v))
			case bool:
				attributes = append(attributes, attribute.Bool(k, v))
			}
		}
	}
	return attributes
}

func (c *Config) GetTelemetryAttributes() []attribute.KeyValue {
	var attributes []attribute.KeyValue

	if len(c.TelemetryAttributes) > 0 {
		for k, t := range c.TelemetryAttributes {
			switch v := t.(type) {
			case string:
				attributes = append(attributes, attribute.String(k, v))
			case bool:
				attributes = append(attributes, attribute.Bool(k, v))
			}
		}
	}
	return attributes
}

func (c *Config) GetHeaders() map[string]string {
	m := make(map[string]string, len(c.Headers))

	for k, t := range c.Headers {
		switch v := t.(type) {
		case bool:
			m[k] = strconv.FormatBool(v)
		case string:
			m[k] = v
		}
	}

	return m
}

// CommonFlags registers common config flags.
func (c *Config) CommonFlags(fs *pflag.FlagSet) {
	fs.IntVar(&c.WorkerCount, "workers", 1, "Number of workers (goroutines) to run")
	fs.Float64Var(&c.Rate, "rate", 0, "Approximately how many metrics/spans/logs per second each worker should generate. Zero means no throttling.")
	fs.DurationVar(&c.TotalDuration, "duration", 0, "For how long to run the test")
	fs.DurationVar(&c.ReportingInterval, "interval", 1*time.Second, "Reporting interval")

	fs.StringVar(&c.CustomEndpoint, "otlp-endpoint", "", "Destination endpoint for exporting logs, metrics and traces")
	fs.BoolVar(&c.Insecure, "otlp-insecure", false, "Whether to enable client transport security for the exporter's grpc or http connection")
	fs.BoolVar(&c.InsecureSkipVerify, "otlp-insecure-skip-verify", false, "Whether a client verifies the server's certificate chain and host name")
	fs.BoolVar(&c.UseHTTP, "otlp-http", false, "Whether to use HTTP exporter rather than a gRPC one")

	// custom headers
	c.Headers = make(KeyValue)
	fs.Var(&c.Headers, "otlp-header", "Custom header to be passed along with each OTLP request. The value is expected in the format key=\"value\". "+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple headers (e.g --otlp-header key1=\"value1\" --otlp-header key2=\"value2\")`)

	// custom resource attributes
	c.ResourceAttributes = make(KeyValue)
	fs.Var(&c.ResourceAttributes, "otlp-attributes", "Custom resource attributes to use. The value is expected in the format key=\"value\". "+
		"You can use key=true or key=false. to set boolean attribute."+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple attributes (e.g --otlp-attributes key1=\"value1\" --otlp-attributes key2=\"value2\" --telemetry-attributes key3=true)`)

	c.TelemetryAttributes = make(KeyValue)
	fs.Var(&c.TelemetryAttributes, "telemetry-attributes", "Custom telemetry attributes to use. The value is expected in the format key=\"value\". "+
		"You can use key=true or key=false. to set boolean attribute."+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple attributes (e.g --telemetry-attributes key1=\"value1\" --telemetry-attributes key2=\"value2\" --telemetry-attributes key3=true)`)

	// TLS CA configuration
	fs.StringVar(&c.CaFile, "ca-cert", "", "Trusted Certificate Authority to verify server certificate")

	// mTLS configuration
	fs.BoolVar(&c.ClientAuth.Enabled, "mtls", false, "Whether to require client authentication for mTLS")
	fs.StringVar(&c.ClientAuth.ClientCertFile, "client-cert", "", "Client certificate file")
	fs.StringVar(&c.ClientAuth.ClientKeyFile, "client-key", "", "Client private key file")
}
