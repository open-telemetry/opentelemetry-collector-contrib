// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/attribute"
	conventions "go.opentelemetry.io/otel/semconv/v1.43.0"

	types "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/pkg"
)

var (
	errFormatOTLPAttributes       = errors.New("value should be in one of the following formats: key=\"value\", key=true, key=false, key=<integer>, key=[<value1>, <value2>, ...] or key={a=\"x\", b=1, c=true, ...}")
	errDoubleQuotesOTLPAttributes = errors.New("value should be a string wrapped in double quotes")
	errMixedTypeSlice             = errors.New("all items in a slice should be of the same type")
	errEmptySlice                 = errors.New("slice should not be empty")
	errEmptyMap                   = errors.New("map should not be empty")
	errUnbalancedBraces           = errors.New("unbalanced braces in map value")
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

func parseAny(val string) (any, error) {
	switch {
	case strings.HasPrefix(val, "{") && strings.HasSuffix(val, "}"):
		content := strings.TrimSpace(val[1 : len(val)-1])
		if content == "" {
			return nil, errEmptyMap
		}
		items, err := splitItems(content)
		if err != nil {
			return nil, err
		}
		return mapFrom(items)
	case strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]"):
		content := strings.TrimSpace(val[1 : len(val)-1])
		if content == "" {
			return nil, errEmptySlice
		}
		items, err := splitItems(content)
		if err != nil {
			return nil, err
		}
		return sliceFrom(items)
	default:
		return parseValue(val)
	}
}

// splitItems splits a string of items separated by commas, taking into account nested structures and quoted strings.
func splitItems(content string) ([]string, error) {
	depth := 0

	var items []string
	var current strings.Builder
	inQuotes := false

	for _, char := range content {
		switch char {
		case '{', '[':
			if !inQuotes {
				depth++
			}
			current.WriteRune(char)
		case '}', ']':
			if !inQuotes {
				depth--
			}
			current.WriteRune(char)
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case ',':
			if !inQuotes && depth == 0 {
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

	if depth != 0 {
		// Unbalanced braces or brackets
		return nil, errUnbalancedBraces
	}

	// Add the last item
	if item := strings.TrimSpace(current.String()); item != "" {
		items = append(items, item)
	}
	return items, nil
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

// mapFrom converts items into a map of key-value pairs
func mapFrom(items []string) (map[string]any, error) {
	result := make(map[string]any)
	for _, item := range items {
		kv := strings.SplitN(item, "=", 2)
		if len(kv) != 2 {
			return nil, errFormatOTLPAttributes
		}
		key := strings.TrimSpace(kv[0])
		val, err := parseAny(strings.TrimSpace(kv[1]))
		if err != nil {
			return nil, err
		}
		result[key] = val
	}
	return result, nil
}

func (v *KeyValue) Set(s string) error {
	kv := strings.SplitN(s, "=", 2)
	if len(kv) != 2 {
		return errFormatOTLPAttributes
	}
	key := strings.TrimSpace(kv[0])

	val, err := parseAny(kv[1])
	if err != nil {
		return err
	}
	(*v)[key] = val

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
	Timeout               time.Duration

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

	// Batching configuration
	Batch     bool
	BatchSize int
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

// toValue converts an interface{} to an attribute.Value, returning a boolean indicating success.
func toValue(v any) (attribute.Value, bool) {
	switch v := v.(type) {
	case string:
		return attribute.StringValue(v), true
	case bool:
		return attribute.BoolValue(v), true
	case int:
		return attribute.IntValue(v), true
	case []string:
		return attribute.StringSliceValue(v), true
	case []bool:
		return attribute.BoolSliceValue(v), true
	case []int:
		return attribute.IntSliceValue(v), true
	case map[string]any:
		kvs := make([]attribute.KeyValue, 0, len(v))
		for k, inner := range v {
			if val, ok := toValue(inner); ok {
				kvs = append(kvs, attribute.KeyValue{Key: attribute.Key(k), Value: val})
			}
		}
		return attribute.MapValue(kvs...), true
	default:
		return attribute.Value{}, false
	}
}

func (c *Config) GetAttributes() []attribute.KeyValue {
	var attributes []attribute.KeyValue

	// may be overridden by `--otlp-attributes service.name="foo"`
	attributes = append(attributes, conventions.ServiceNameKey.String(c.ServiceName))
	if len(c.ResourceAttributes) > 0 {
		for k, t := range c.ResourceAttributes {
			if val, ok := toValue(t); ok {
				attributes = append(attributes, attribute.KeyValue{Key: attribute.Key(k), Value: val})
			}
		}
	}
	return attributes
}

func (c *Config) GetTelemetryAttributes() []attribute.KeyValue {
	var attributes []attribute.KeyValue

	if len(c.TelemetryAttributes) > 0 {
		for k, t := range c.TelemetryAttributes {
			if val, ok := toValue(t); ok {
				attributes = append(attributes, attribute.KeyValue{Key: attribute.Key(k), Value: val})
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
	fs.DurationVar(&c.Timeout, "timeout", c.Timeout, "Maximum time to wait for the signals to reach destination.")

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
	fs.Var(&c.ResourceAttributes, "otlp-attributes", "Custom resource attributes, describing the entity that produces the telemetry. They are set once on the OTLP resource and are shared by every span, metric data point and log record emitted by this run, e.g. --otlp-attributes service.name=\"my-service\" overrides --service. "+
		"The value is expected in one of the following formats: key=\"value\", key=true, key=false, key=<integer>, or key=[\"value1\", \"value2\", ...]. "+
		"Note you may need to escape the quotes when using the tool from a cli. "+
		`Flag may be repeated to set multiple attributes (e.g --otlp-attributes key1=\"value1\" --otlp-attributes key2=\"value2\" --otlp-attributes key3=true --otlp-attributes key4=123 --otlp-attributes key5=[1,2,3])`)

	fs.Var(&c.TelemetryAttributes, "telemetry-attributes", "Custom attributes describing an individual telemetry item. They are set on every generated span, metric data point and log record rather than on the resource, e.g. --telemetry-attributes http.response.status_code=200. "+
		"The value is expected in one of the following formats: key=\"value\", key=true, key=false, or key=<integer>, or key=[\"value1\", \"value2\", ...]. "+
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

	// Batching configuration
	fs.BoolVar(&c.Batch, "batch", c.Batch, "Whether to batch telemetry records before sending")
	fs.IntVar(&c.BatchSize, "batch-size", c.BatchSize, "Number of telemetry records to batch before sending")
}

// SetDefaults is here to mirror the defaults for flags above,
// This allows for us to have a single place to change the defaults
// while exposing the API for use.
func (c *Config) SetDefaults() {
	c.WorkerCount = 1
	c.Rate = 0
	c.TotalDuration = types.DurationWithInf(0)
	c.ReportingInterval = 1 * time.Second
	c.Timeout = 10 * time.Second
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
	c.Batch = true
	c.BatchSize = 100
}

// CharactersPerMB is the number of characters needed to create a 1MB string attribute
const CharactersPerMB = 1024 * 1024

// CreateLoadAttribute creates a string attribute with the specified size in MB
// This is commonly used across different signal types (metrics, traces, logs) for load testing
func CreateLoadAttribute(key string, sizeMB int) attribute.KeyValue {
	return attribute.String(key, string(make([]byte, CharactersPerMB*sizeMB)))
}
