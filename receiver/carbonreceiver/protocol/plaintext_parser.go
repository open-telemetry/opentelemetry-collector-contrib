// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"

import (
	"fmt"
	"strings"
)

// PlaintextConfig holds the configuration for the plaintext parser.
type PlaintextConfig struct{}

var _ (ParserConfig) = (*PlaintextConfig)(nil)

// BuildParser creates a new Parser instance that receives plaintext
// Carbon data.
func (p *PlaintextConfig) BuildParser() (Parser, error) {
	pathParser := &PlaintextPathParser{}
	return NewParser(pathParser)
}

// PlaintextPathParser converts a line of https://graphite.readthedocs.io/en/latest/feeding-carbon.html#the-plaintext-protocol,
// treating tags per spec at https://graphite.readthedocs.io/en/latest/tags.html#carbon.
type PlaintextPathParser struct{}

// ParsePath converts the <metric_path> of a Carbon line (see Parse function for
// description of the full line). The metric path is expected to be in the
// following format:
//
//	<metric_name>[;tag0;...;tagN]
//
// <metric_name> is the name of the metric and terminates either at the first ';'
// or at the end of the path.
//
// tag is of the form "key=val", where key can contain any char except ";!^=" and
// val can contain any char except ";~".
func (p *PlaintextPathParser) ParsePath(path string, parsedPath *ParsedPath) error {
	parts := strings.SplitN(path, ";", 2)
	if len(parts) < 1 || parts[0] == "" {
		return fmt.Errorf("empty metric name extracted from path [%s]", path)
	}

	parsedPath.MetricName = parts[0]
	if len(parts) == 1 {
		// No tags, no more work here.
		return nil
	}

	if parts[1] == "" {
		// Empty tags, nothing to do.
		return nil
	}

	tags := strings.Split(parts[1], ";")
	attributes := map[string]any{}
	for _, tag := range tags {
		idx := strings.IndexByte(tag, '=')
		if idx < 1 {
			return fmt.Errorf("cannot parse metric path [%s]: incorrect key value separator for [%s]", path, tag)
		}

		key := tag[:idx]
		value := tag[idx+1:] // If value is empty, ie.: tag == "k=", this will return "".
		attributes[key] = value
	}
	parsedPath.Attributes = attributes

	return nil
}

func plaintextDefaultConfig() ParserConfig {
	return &PlaintextConfig{}
}
