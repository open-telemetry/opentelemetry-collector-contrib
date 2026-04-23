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

// EncodingName returns the encoding's name; satisfies the encodingEntry interface.
func (e S3Encoding) EncodingName() string { return e.Name }

// Validate validates an S3Encoding entry.
func (e S3Encoding) Validate() error {
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

// encodingEntry is the contract shared by S3Encoding and CWEncoding for
// validateEncodingList: each entry must be self-validating and expose its name.
type encodingEntry interface {
	Validate() error
	EncodingName() string
}

// validateEncodingList performs the validation shared by S3Config and
// CloudWatchConfig: single encoding and encodings list are mutually exclusive,
// every entry validates, no two entries share a name.
func validateEncodingList[T encodingEntry](encoding string, encodings []T) error {
	if encoding != "" && len(encodings) > 0 {
		return errors.New("'encoding' and 'encodings' are mutually exclusive; use 'encodings' for multi-format support")
	}
	seen := make(map[string]bool, len(encodings))
	for i, e := range encodings {
		if err := e.Validate(); err != nil {
			return fmt.Errorf("encodings[%d]: %w", i, err)
		}
		name := e.EncodingName()
		if seen[name] {
			return fmt.Errorf("encodings[%d]: duplicate encoding name %q", i, name)
		}
		seen[name] = true
	}
	return nil
}

// defaultCWPatterns maps known CloudWatch encoding names to their default log
// group / log stream patterns. Patterns are expressed using the shared
// match.go syntax (exact, "*", or affix wildcard).
var defaultCWPatterns = map[string]CWEncoding{
	"vpcflow":    {Name: "vpcflow", LogStreamPattern: "eni-*"},
	"cloudtrail": {Name: "cloudtrail", LogStreamPattern: "*_CloudTrail_*"},
	"lambda":     {Name: "lambda", LogGroupPattern: "/aws/lambda/*"},
	"waf":        {Name: "waf", LogGroupPattern: "aws-waf-logs-*"},
	"rds":        {Name: "rds", LogGroupPattern: "/aws/rds/instance/*/*"},
	"eks":        {Name: "eks", LogGroupPattern: "/aws/eks/*"},
	"apigateway": {Name: "apigateway", LogGroupPattern: "API-Gateway-Execution-Logs_*"},
}

// CWEncoding defines one entry in the CloudWatch multi-encoding routing table.
// Each entry maps a log group and/or log stream pattern to an encoding extension.
type CWEncoding struct {
	// Name identifies the encoding. For known names (vpcflow, cloudtrail,
	// lambda, waf, rds, eks, apigateway) the default log_group_pattern or
	// log_stream_pattern is applied automatically when neither is set.
	Name string `mapstructure:"name"`

	// Encoding is the extension ID for decoding. If empty, content is passed
	// through as-is using the built-in raw CloudWatch decoder.
	Encoding string `mapstructure:"encoding"`

	// LogGroupPattern is matched against the subscription event's logGroup.
	// Uses the shared match syntax: exact, "*", or affix wildcards.
	// Example: "/aws/lambda/*".
	LogGroupPattern string `mapstructure:"log_group_pattern"`

	// LogStreamPattern is matched against the subscription event's logStream.
	// Uses the shared match syntax. Example: "eni-*".
	LogStreamPattern string `mapstructure:"log_stream_pattern"`
}

// withDefaults returns a copy of the encoding with default patterns applied.
// Defaults are only applied when the user supplied neither log_group_pattern
// nor log_stream_pattern; user-supplied patterns are never overwritten.
func (e CWEncoding) withDefaults() CWEncoding {
	if e.LogGroupPattern != "" || e.LogStreamPattern != "" {
		return e
	}
	if d, ok := defaultCWPatterns[e.Name]; ok {
		e.LogGroupPattern = d.LogGroupPattern
		e.LogStreamPattern = d.LogStreamPattern
	}
	return e
}

// EncodingName returns the encoding's name; satisfies the encodingEntry interface.
func (e CWEncoding) EncodingName() string { return e.Name }

// Validate validates a CWEncoding entry.
func (e CWEncoding) Validate() error {
	if e.Name == "" {
		return errors.New("'name' is required")
	}
	if e.LogGroupPattern == "" && e.LogStreamPattern == "" {
		if _, ok := defaultCWPatterns[e.Name]; !ok {
			return fmt.Errorf("'log_group_pattern' or 'log_stream_pattern' is required for encoding %q (no default available); use %q for catch-all", e.Name, catchAllPattern)
		}
	}
	return nil
}

// CloudWatchConfig defines configuration options for the CloudWatch Logs
// Lambda trigger. sharedConfig is embedded via squash so the "encoding" key
// remains at the top level of the cloudwatch block — fully backwards compatible.
type CloudWatchConfig struct {
	sharedConfig `mapstructure:",squash"`

	// Encodings defines multiple encoding entries for CloudWatch log-group /
	// log-stream routing (multi-format mode). Mutually exclusive with
	// sharedConfig.Encoding.
	//
	// Only supported for the logs signal type. Metrics receivers reject
	// configs that set this field.
	Encodings []CWEncoding `mapstructure:"encodings"`
}

// Validate validates the CloudWatchConfig.
func (c *CloudWatchConfig) Validate() error {
	return validateEncodingList(c.Encoding, c.Encodings)
}

// sortedEncodings returns a copy of Encodings sorted by specificity. Three-level sort:
//  1. Catch-all entries ("*") always last.
//  2. Entries with log_group_pattern before entries with only log_stream_pattern.
//  3. Within each group, more-specific pattern first (via comparePatternSpecificity).
//
// Defaults are applied before sorting via withDefaults.
func (c *CloudWatchConfig) sortedEncodings() []CWEncoding {
	if len(c.Encodings) == 0 {
		return nil
	}
	sorted := make([]CWEncoding, len(c.Encodings))
	for i, e := range c.Encodings {
		sorted[i] = e.withDefaults()
	}
	// Pre-split non-empty patterns once; keyed by pattern string so the map
	// stays correct as the sort swaps elements.
	splitCache := make(map[string][]string, len(sorted)*2)
	for _, e := range sorted {
		for _, p := range [...]string{e.LogGroupPattern, e.LogStreamPattern} {
			if p == "" {
				continue
			}
			if _, ok := splitCache[p]; !ok {
				splitCache[p] = strings.Split(p, "/")
			}
		}
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return compareCWEncodings(sorted[i], sorted[j], splitCache) < 0
	})
	return sorted
}

// compareCWEncodings implements the three-level sort for CloudWatch encodings.
func compareCWEncodings(a, b CWEncoding, splitCache map[string][]string) int {
	// Level 1: catch-all "*" always last.
	aIsCatchAll := isCatchAllPattern(a.LogGroupPattern) || isCatchAllPattern(a.LogStreamPattern)
	bIsCatchAll := isCatchAllPattern(b.LogGroupPattern) || isCatchAllPattern(b.LogStreamPattern)
	if aIsCatchAll && !bIsCatchAll {
		return 1
	}
	if !aIsCatchAll && bIsCatchAll {
		return -1
	}
	// Level 2: log_group_pattern before log_stream_pattern-only entries.
	aHasGroup := a.LogGroupPattern != ""
	bHasGroup := b.LogGroupPattern != ""
	if aHasGroup && !bHasGroup {
		return -1
	}
	if !aHasGroup && bHasGroup {
		return 1
	}
	// Level 3: more-specific pattern first.
	var patA, patB string
	if aHasGroup {
		patA, patB = a.LogGroupPattern, b.LogGroupPattern
	} else {
		patA, patB = a.LogStreamPattern, b.LogStreamPattern
	}
	return comparePatternSpecificity(splitCache[patA], splitCache[patB])
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
	return validateEncodingList(c.Encoding, c.Encodings)
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
	CloudWatch CloudWatchConfig `mapstructure:"cloudwatch"`

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
	if err := c.CloudWatch.Validate(); err != nil {
		return fmt.Errorf("invalid cloudwatch config: %w", err)
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
