// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awslambdareceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awslambdareceiver"

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/collector/component"
)

const s3ARNPrefix = "arn:aws:s3:::"

// defaultS3PathPatterns maps known encoding names to their default S3 path patterns.
// "*" matches exactly one path segment (the AWS account ID in standard AWS log paths).
var defaultS3PathPatterns = map[string]string{
	"vpcflow":         "AWSLogs/*/vpcflowlogs",
	"cloudtrail":      "AWSLogs/*/CloudTrail",
	"elbaccess":       "AWSLogs/*/elasticloadbalancing",
	"waf":             "AWSLogs/*/WAFLogs",
	"networkfirewall": "AWSLogs/*/network-firewall",
}

// S3Encoding defines one entry in the S3 multi-encoding routing table.
type S3Encoding struct {
	// Name identifies the encoding. For known names (vpcflow, cloudtrail, elbaccess, waf,
	// networkfirewall) the default path_pattern is applied automatically.
	Name string `mapstructure:"name"`

	// Encoding is the extension ID for decoding (e.g. "awslogs_encoding/vpcflow").
	// If empty, content is passed through as-is using the built-in raw decoder.
	Encoding string `mapstructure:"encoding"`

	// PathPattern is matched as a prefix against the S3 object key.
	// "*" matches exactly one path segment. Example: "AWSLogs/*/vpcflowlogs"
	// If empty, the default pattern for a known Name is used.
	PathPattern string `mapstructure:"path_pattern"`
}

// resolvePathPattern returns the effective path pattern for this encoding entry.
// Returns the configured PathPattern if set, else the default for known names.
func (e *S3Encoding) resolvePathPattern() string {
	if e.PathPattern != "" {
		return e.PathPattern
	}
	return defaultS3PathPatterns[e.Name]
}

// Validate validates an S3Encoding entry.
func (e *S3Encoding) Validate() error {
	if e.Name == "" {
		return errors.New("'name' is required")
	}
	// Unknown name without an explicit path_pattern is not routable.
	if e.PathPattern == "" {
		if _, ok := defaultS3PathPatterns[e.Name]; !ok {
			return fmt.Errorf("'path_pattern' is required for encoding %q (no default available); use %q for catch-all", e.Name, catchAllPattern)
		}
	}
	return nil
}

// sharedConfig defines configuration options shared between Lambda trigger types.
type sharedConfig struct {
	// Encoding defines the encoding to decode incoming Lambda invocation data.
	Encoding string `mapstructure:"encoding"`
}

// S3Config defines configuration options for the S3 Lambda trigger.
// It supersedes the sharedConfig for the s3 key.
// sharedConfig is embedded with mapstructure:",squash" so the "encoding" key
// remains at the top level of the s3 block in YAML — fully backwards-compatible.
type S3Config struct {
	sharedConfig `mapstructure:",squash"`

	// Encodings defines multiple encoding entries for S3 path-based routing (multi-format mode).
	// Each entry maps a path_pattern prefix to an encoding extension.
	// Mutually exclusive with sharedConfig.Encoding.
	//
	// Only supported for logs signal type. Metrics receivers reject configs that set this field.
	Encodings []S3Encoding `mapstructure:"encodings"`
}

// Validate validates the S3Config.
func (c *S3Config) Validate() error {
	if c.Encoding != "" && len(c.Encodings) > 0 {
		return errors.New("'encoding' and 'encodings' are mutually exclusive; use 'encodings' for multi-format support")
	}
	seen := make(map[string]bool, len(c.Encodings))
	for i, e := range c.Encodings {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("encodings[%d]: %w", i, err)
		}
		if seen[e.Name] {
			return fmt.Errorf("encodings[%d]: duplicate encoding name %q", i, e.Name)
		}
		seen[e.Name] = true
	}
	return nil
}

// sortedEncodings returns a copy of Encodings sorted by path pattern specificity:
// more-specific patterns first, catch-all "*" last.
// This makes matching order-independent — users can list encodings in any order.
func (c *S3Config) sortedEncodings() []S3Encoding {
	if len(c.Encodings) == 0 {
		return nil
	}
	sorted := make([]S3Encoding, len(c.Encodings))
	copy(sorted, c.Encodings)

	// Pre-split patterns once; keyed by pattern string so the map stays correct
	// as the sort swaps elements.
	splitCache := make(map[string][]string, len(sorted))
	for _, enc := range sorted {
		p := enc.resolvePathPattern()
		if _, ok := splitCache[p]; !ok {
			splitCache[p] = strings.Split(p, "/")
		}
	}

	sort.SliceStable(sorted, func(i, j int) bool {
		pi := sorted[i].resolvePathPattern()
		pj := sorted[j].resolvePathPattern()
		if isCatchAllPattern(pi) && !isCatchAllPattern(pj) {
			return false
		}
		if !isCatchAllPattern(pi) && isCatchAllPattern(pj) {
			return true
		}
		return comparePatternSpecificity(splitCache[pi], splitCache[pj]) < 0
	})
	return sorted
}

// Config is the top-level configuration for the awslambda receiver.
type Config struct {
	// S3 defines configuration for the S3 Lambda trigger.
	S3 S3Config `mapstructure:"s3"`

	// CloudWatch defines configuration for the CloudWatch Logs Lambda trigger.
	CloudWatch sharedConfig `mapstructure:"cloudwatch"`

	// FailureBucketARN is the ARN of the S3 bucket used to store failed Lambda event records.
	FailureBucketARN string `mapstructure:"failure_bucket_arn"`

	_ struct{} // Prevent unkeyed literal initialization.
}

var _ component.Config = (*Config)(nil)

func createDefaultConfig() component.Config {
	return &Config{}
}

func (c *Config) Validate() error {
	if c.FailureBucketARN != "" {
		if _, err := getBucketNameFromARN(c.FailureBucketARN); err != nil {
			return fmt.Errorf("invalid failure_bucket_arn: %w", err)
		}
	}
	if err := c.S3.Validate(); err != nil {
		return fmt.Errorf("invalid s3 config: %w", err)
	}
	return nil
}

// getBucketNameFromARN extracts the S3 bucket name from an ARN.
// Example: "arn:aws:s3:::myBucket/folderA" => "myBucket"
func getBucketNameFromARN(arn string) (string, error) {
	if !strings.HasPrefix(arn, s3ARNPrefix) {
		return "", fmt.Errorf("invalid S3 ARN format: %s", arn)
	}
	s3Path := strings.TrimPrefix(arn, s3ARNPrefix)
	bucket, _, _ := strings.Cut(s3Path, "/")
	if bucket == "" {
		return "", fmt.Errorf("invalid S3 ARN format, bucket name missing: %s", arn)
	}
	return bucket, nil
}
