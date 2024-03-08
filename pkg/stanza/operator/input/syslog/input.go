// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslog // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/syslog"

import (
	"bufio"
	"regexp"
	"strconv"

	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/parser/syslog"
)

// Input is an operator that listens for log entries over tcp.
type Input struct {
	helper.InputOperator
	tcp    *tcp.Input
	udp    *udp.Input
	parser *syslog.Parser
}

// Start will start listening for log entries over tcp or udp.
func (t *Input) Start(p operator.Persister) error {
	if t.tcp != nil {
		return t.tcp.Start(p)
	}
	return t.udp.Start(p)
}

// Stop will stop listening for messages.
func (t *Input) Stop() error {
	if t.tcp != nil {
		return t.tcp.Stop()
	}
	return t.udp.Stop()
}

// SetOutputs will set the outputs of the internal syslog parser.
func (t *Input) SetOutputs(operators []operator.Operator) error {
	t.parser.SetOutputIDs(t.GetOutputIDs())
	return t.parser.SetOutputs(operators)
}

func OctetSplitFuncBuilder(_ encoding.Encoding) (bufio.SplitFunc, error) {
	return newOctetFrameSplitFunc(true), nil
}

func newOctetFrameSplitFunc(flushAtEOF bool) bufio.SplitFunc {
	frameRegex := regexp.MustCompile(`^[1-9]\d*\s`)
	return func(data []byte, atEOF bool) (int, []byte, error) {
		frameLoc := frameRegex.FindIndex(data)
		if frameLoc == nil {
			// Flush if no more data is expected
			if len(data) != 0 && atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		}

		frameMaxIndex := frameLoc[1]
		// Remove the delimiter (space) between length and log, and parse the length
		frameLenValue, err := strconv.Atoi(string(data[:frameMaxIndex-1]))
		if err != nil {
			// This should not be possible because the regex matched.
			// However, return an error just in case.
			return 0, nil, err
		}

		advance := frameMaxIndex + frameLenValue
		if advance > len(data) {
			if atEOF && flushAtEOF {
				return len(data), data, nil
			}
			return 0, nil, nil
		}
		return advance, data[:advance], nil
	}
}
