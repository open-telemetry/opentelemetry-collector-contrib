// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

var (
	errFormatOTLPAttributes       = errors.New("value should be in one of the following formats: key=\"value\", key=true, key=false, key=<integer>, or key=[<value1>, <value2>, ...]")
	errDoubleQuotesOTLPAttributes = errors.New("value should be a string wrapped in double quotes")
	errMixedTypeSlice             = errors.New("all items in a slice should be of the same type")
	errEmptySlice                 = errors.New("slice should not be empty")
)

const (
	defaultGRPCEndpoint = "localhost:4317"
	defaultHTTPEndpoint = "localhost:4318"
)

type KeyValue map[string]any

var _ pflag.Value = (*KeyValue)(nil)

func (*KeyValue) String() string {
	return ""
}

func parseValue(val string) (any, error) {
	if val == "true" {
		return true, nil
	}
	if val == "false" {
		return false, nil
	}
	if intVal, err := strconv.Atoi(val); err == nil {
		return intVal, nil
	}
	if len(val) < 2 || !strings.HasPrefix(val, "\"") || !strings.HasSuffix(val, "\"") {
		return nil, errDoubleQuotesOTLPAttributes
	}
	return val[1 : len(val)-1], nil
}

// splitItems splits the pre-trimmed content into a list of items separated by commas, respecting quotes
func splitItems(content string) []string {
	var items []string
	var current strings.Builder
	inQuotes := false

	for _, char := range content {
		switch char {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case ',':
			if !inQuotes {
				if item := strings.TrimSpace(current.String()); item != "" {
					items = append(items, item)
				}
				current.Reset()
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}
	// Add the last item
	if item := strings.TrimSpace(current.String()); item != "" {
		items = append(items, item)
	}
	return items
}

// sliceFrom converts items into a slice of a single type
func sliceFrom(items []string) (any, error) {
	firstItem, err := parseValue(items[0])
	if err != nil {
		return nil, err
	}
	sliceType := reflect.TypeOf(firstItem)
	slice := reflect.MakeSlice(reflect.SliceOf(sliceType), len(items), len(items))
	for i, item := range items {
		val, err := parseValue(item)
		if err != nil {
			return nil, err
		}
		if reflect.TypeOf(val) != sliceType {
			return nil, errMixedTypeSlice
		}
		slice.Index(i).Set(reflect.ValueOf(val))
	}
	return slice.Interface(), nil
}

func (v *KeyValue) Set(s string) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return errFormatOTLPAttributes
	}
	key := kv[0]

	if !strings.HasPrefix(kv[1], "[") || !strings.HasSuffix(kv[1], "]") {
		val, err := parseValue(kv[1])
		if err != nil {
			return err
		}
		(*v)[key] = val
		return nil
	}

	// List of Values
	content := strings.TrimSpace(kv[1][1 : len(kv[1])-1])
	if content == "" {
		return errEmptySlice
	}

	items := splitItems(content)
	slice, err := sliceFrom(items)
	if err != nil {
		return err
	}
	(*v)[key] = slice
	return nil
}

func (*KeyValue) Type() string {
	return "map[string]any"
}

type Config struct {
	WorkerCount           int
	Rate                  float64
	TotalDuration         types.DurationWithInf
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
	ServiceName         string
	TelemetryAttributes KeyValue

	// OTLP TLS configuration
	CaFile string

	// OTLP mTLS configuration
	ClientAuth ClientAuth

	// Export behavior configuration
	AllowExportFailures bool

	// Load testing configuration
	LoadSize int
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

	// may be overridden by `--otlp-attributes service.name="foo"`
	attributes = append(attributes, semconv.ServiceNameKey.String(c.ServiceName))
	if len(c.ResourceAttributes) > 0 {
		for k, t := range c.ResourceAttributes {
			switch v := t.(type) {
			case string:
				attributes = append(attributes, attribute.String(k, v))
			case bool:
				attributes = append(attributes, attribute.Bool(k, v))
			case int:
				attributes = append(attributes, attribute.Int(k, v))
			case []string:
				attributes = append(attributes, attribute.StringSlice(k, v))
			case []bool:
				attributes = append(attributes, attribute.BoolSlice(k, v))
			case []int:
				attributes = append(attributes, attribute.IntSlice(k, v))
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
			case int:
				attributes = append(attributes, attribute.Int(k, v))
			case []string:
				attributes = append(attributes, attribute.StringSlice(k, v))
			case []bool:
				attributes = append(attributes, attribute.BoolSlice(k, v))
			case []int:
				attributes = append(attributes, attribute.IntSlice(k, v))
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
	fs.IntVar(&c.WorkerCount, "workers", c.WorkerCount, "Number of workers (goroutines) to run")
	fs.Float64Var(&c.Rate, "rate", c.Rate, "Approximately how many metrics/spans/logs per second each worker should generate. Zero means no throttling.")
	fs.Var(&c.TotalDuration, "duration", "For how long to run the test. Use 'inf' for infinite duration.")
	fs.DurationVar(&c.ReportingInterval, "interval", c.ReportingInterval, "Reporting interval")

	fs.StringVar(&c.CustomEndpoint, "otlp-endpoint", c.CustomEndpoint, "Destination endpoint for exporting logs, metrics and traces")
	fs.BoolVar(&c.Insecure, "otlp-insecure", c.Insecure, "Whether to enable client transport security for the exporter's grpc or http connection")
	fs.BoolVar(&c.InsecureSkipVerify, "otlp-insecure-skip-verify", c.InsecureSkipVerify, "Whether a client verifies the server's certificate chain and host name")
	fs.BoolVar(&c.UseHTTP, "otlp-http", c.UseHTTP, "Whether to use HTTP exporter rather than a gRPC one")

	fs.StringVar(&c.ServiceName, "service", c.ServiceName, "Service name to use")

	// custom headers
	fs.Var(&c.Headers, "otlp-header", "Custom header to be passed along with each OTLP request. The value is expected in the format key=\"value\". "+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple headers (e.g --otlp-header key1=\"value1\" --otlp-header key2=\"value2\")`)

	// custom resource attributes
	fs.Var(&c.ResourceAttributes, "otlp-attributes", "Custom telemetry attributes to use. The value is expected in one of the following formats: key=\"value\", key=true, key=false, key=<integer>, or key=[\"value1\", \"value2\", ...]. "+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple attributes (e.g --otlp-attributes key1=\"value1\" --otlp-attributes key2=\"value2\" --otlp-attributes key3=true --otlp-attributes key4=123 --otlp-attributes key5=[1,2,3])`)

	fs.Var(&c.TelemetryAttributes, "telemetry-attributes", "Custom telemetry attributes to use. The value is expected in one of the following formats: key=\"value\", key=true, key=false, or key=<integer>, or key=[\"value1\", \"value2\", ...]. "+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple attributes (e.g --telemetry-attributes key1=\"value1\" --telemetry-attributes key2=\"value2\" --telemetry-attributes key3=true --telemetry-attributes key4=123 --telemetry-attributes key5=[1,2,3])`)

	// TLS CA configuration
	fs.StringVar(&c.CaFile, "ca-cert", c.CaFile, "Trusted Certificate Authority to verify server certificate")

	// mTLS configuration
	fs.BoolVar(&c.ClientAuth.Enabled, "mtls", c.ClientAuth.Enabled, "Whether to require client authentication for mTLS")
	fs.StringVar(&c.ClientAuth.ClientCertFile, "client-cert", c.ClientAuth.ClientCertFile, "Client certificate file")
	fs.StringVar(&c.ClientAuth.ClientKeyFile, "client-key", c.ClientAuth.ClientKeyFile, "Client private key file")

	// Export behavior configuration
	fs.BoolVar(&c.AllowExportFailures, "allow-export-failures", c.AllowExportFailures, "Whether to continue running when export operations fail (instead of terminating)")

	// Load testing configuration
	fs.IntVar(&c.LoadSize, "size", c.LoadSize, "Desired minimum size in MB of string data for each generated telemetry record")
}

// SetDefaults is here to mirror the defaults for flags above,
// This allows for us to have a single place to change the defaults
// while exposing the API for use.
func (c *Config) SetDefaults() {
	c.WorkerCount = 1
	c.Rate = 0
	c.TotalDuration = types.DurationWithInf(0)
	c.ReportingInterval = 1 * time.Second
	c.CustomEndpoint = ""
	c.Insecure = false
	c.InsecureSkipVerify = false
	c.UseHTTP = false
	c.HTTPPath = ""
	c.Headers = make(KeyValue)
	c.ResourceAttributes = make(KeyValue)
	c.ServiceName = "telemetrygen"
	c.TelemetryAttributes = make(KeyValue)
	c.CaFile = ""
	c.ClientAuth.Enabled = false
	c.ClientAuth.ClientCertFile = ""
	c.ClientAuth.ClientKeyFile = ""
	c.AllowExportFailures = false
	c.LoadSize = 0
}

// CharactersPerMB is the number of characters needed to create a 1MB string attribute
const CharactersPerMB = 1024 * 1024

// CreateLoadAttribute creates a string attribute with the specified size in MB
// This is commonly used across different signal types (metrics, traces, logs) for load testing
func CreateLoadAttribute(key string, sizeMB int) attribute.KeyValue {
	return attribute.String(key, string(make([]byte, CharactersPerMB*sizeMB)))
}
