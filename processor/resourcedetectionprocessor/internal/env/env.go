// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package env provides a detector that loads resource information from
// the OTEL_RESOURCE environment variable. A list of labels of the form
// `<key1>=<value1>,<key2>=<value2>,...` is accepted. Domain names and
// paths are accepted as label keys.
package env // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/env"

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

// TypeStr is type of detector.
const TypeStr = "env"

// Environment variable used by "env" to decode a resource.
const envVar = "OTEL_RESOURCE_ATTRIBUTES"

// Deprecated environment variable used by "env" to decode a resource.
// Specification states to use OTEL_RESOURCE_ATTRIBUTES however to avoid
// breaking existing usage, maintain support for OTEL_RESOURCE
// https://github.com/open-telemetry/opentelemetry-specification/blob/1afab39e5658f807315abf2f3256809293bfd421/specification/resource/sdk.md#specifying-resource-information-via-an-environment-variable
const deprecatedEnvVar = "OTEL_RESOURCE"

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	included []*regexp.Regexp
	excluded []*regexp.Regexp
}

func NewDetector(_ processor.Settings, dcfg internal.DetectorConfig, _ bool) (internal.Detector, error) {
	d := &Detector{}
	cfg, ok := dcfg.(Config)
	if !ok {
		return d, nil
	}
	var err error
	if d.included, err = compilePatterns(cfg.Attributes.Included); err != nil {
		return nil, fmt.Errorf("invalid env detector attributes.included pattern: %w", err)
	}
	if d.excluded, err = compilePatterns(cfg.Attributes.Excluded); err != nil {
		return nil, fmt.Errorf("invalid env detector attributes.excluded pattern: %w", err)
	}
	return d, nil
}

// compilePatterns turns each entry into an anchored regexp where `*` matches any
// run of characters. Other regexp metacharacters in the input are escaped.
func compilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		parts := strings.Split(p, "*")
		for i, part := range parts {
			parts[i] = regexp.QuoteMeta(part)
		}
		re, err := regexp.Compile("^" + strings.Join(parts, ".*") + "$")
		if err != nil {
			return nil, fmt.Errorf("%q: %w", p, err)
		}
		out = append(out, re)
	}
	return out, nil
}

func matchAny(patterns []*regexp.Regexp, key string) bool {
	for _, re := range patterns {
		if re.MatchString(key) {
			return true
		}
	}
	return false
}

func (d *Detector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()

	labels := strings.TrimSpace(os.Getenv(envVar))
	if labels == "" {
		labels = strings.TrimSpace(os.Getenv(deprecatedEnvVar))
		if labels == "" {
			return res, "", nil
		}
	}

	if err = initializeAttributeMap(res.Attributes(), labels); err != nil {
		res.Attributes().Clear()
		return res, "", err
	}

	if len(d.included) > 0 || len(d.excluded) > 0 {
		res.Attributes().RemoveIf(func(k string, _ pcommon.Value) bool {
			if len(d.included) > 0 && !matchAny(d.included, k) {
				return true
			}
			return matchAny(d.excluded, k)
		})
	}

	return res, "", nil
}

// labelRegex matches any key=value pair including a trailing comma or the end of the
// string. Captures the trimmed key & value parts, and ignores any superfluous spaces.
var labelRegex = regexp.MustCompile(`\s*([[:ascii:]]{1,256}?)\s*=\s*([[:ascii:]]{0,256}?)\s*(?:,|$)`)

func initializeAttributeMap(am pcommon.Map, s string) error {
	matches := labelRegex.FindAllStringSubmatchIndex(s, -1)
	for len(matches) == 0 {
		return fmt.Errorf("invalid resource format: %q", s)
	}

	prevIndex := 0
	for _, match := range matches {
		// if there is any text between matches, raise an error
		if prevIndex != match[0] {
			return fmt.Errorf("invalid resource format, invalid text: %q", s[prevIndex:match[0]])
		}

		key := s[match[2]:match[3]]
		value := s[match[4]:match[5]]

		var err error
		if value, err = url.QueryUnescape(value); err != nil {
			return fmt.Errorf("invalid resource format in attribute: %q, err: %w", s[match[0]:match[1]], err)
		}
		am.PutStr(key, value)

		prevIndex = match[1]
	}

	// if there is any text after the last match, raise an error
	if matches[len(matches)-1][1] != len(s) {
		return fmt.Errorf("invalid resource format, invalid text: %q", s[matches[len(matches)-1][1]:])
	}

	return nil
}
