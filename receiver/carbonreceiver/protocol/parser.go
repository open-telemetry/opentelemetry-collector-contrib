// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"

import (
	"go.opentelemetry.io/collector/pdata/pmetric"
)

// Parser abstracts the type of parsing being done by the receiver.
type Parser interface {
	// Parse receives the string with plaintext data, aka line, in the Carbon
	// format and transforms it to the collector metric format.
	//
	// The expected line is a text line in the following format:
	// 	"<metric_path> <metric_value> <metric_timestamp>"
	//
	// The <metric_path> is where there are variations that require selection
	// of specialized parsers to handle them, but include the metric name and
	// labels/dimensions for the metric.
	//
	// The <metric_value> is the textual representation of the metric value.
	//
	// The <metric_timestamp> is the Unix time text of when the measurement was
	// made.
	Parse(line string) (pmetric.Metric, error)
}
