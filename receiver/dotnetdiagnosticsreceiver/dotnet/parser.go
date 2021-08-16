// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dotnet

import (
	"context"
	"errors"
	"io"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver/network"
)

// Parser encapsulates all of the functionality to parse an IPC stream.
type Parser struct {
	r       network.MultiReader
	consume func([]Metric)
	logger  *zap.Logger
}

// MetricsConsumer is a function that accepts a slice of Metrics. Parser has a
// member consumer function, used to send Metrics as they are created.
type MetricsConsumer func([]Metric)

// NewParser accepts an io.Reader, a MetricsConsumer, and logger, and returns a
// Parser for processing an IPC stream.
func NewParser(rdr io.Reader, mc MetricsConsumer, bw network.BlobWriter, logger *zap.Logger) *Parser {
	r := network.NewMultiReader(rdr, bw)
	return &Parser{r: r, consume: mc, logger: logger}
}

// ParseIPC parses the IPC response from the initial request to a dotnet process.
func (p *Parser) ParseIPC() error {
	return parseIPC(p.r)
}

// ParseNettrace parses the nettrace magic message.
func (p *Parser) ParseNettrace() error {
	return parseNettrace(p.r)
}

// ParseAll parses all of the blocks until an error occurs or the context is
// cancelled.
func (p *Parser) ParseAll(ctx context.Context) error {
	var err error
	fms := fieldMetadataMap{}
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			err = p.parseBlock(fms)
			// flush regardless of error
			p.r.Flush()
			if err != nil {
				return err
			}
		}
	}
}

// parseBlock parses a serialization type and block, bookended by a begin and
// end object tag.
func (p *Parser) parseBlock(fms fieldMetadataMap) error {
	err := beginPrivateObject(p.r)
	if err != nil {
		return err
	}

	st, err := parseSerializationType(p.r)
	if err != nil {
		return err
	}

	p.logger.Debug("parsing block", zap.String("serialization type", st.name))

	switch st.name {
	case "Trace":
		err = parseTraceMessage(p.r)
		if err != nil {
			return err
		}
	case "MetadataBlock":
		err = parseMetadataBlock(p.r, fms)
		if err != nil {
			return err
		}
	case "StackBlock":
		err = parseStackBlock(p.r)
		if err != nil {
			return err
		}
	case "EventBlock":
		var metrics []Metric
		metrics, err = parseEventBlock(p.r, fms)
		if err != nil {
			return err
		}
		p.consume(metrics)
	case "SPBlock":
		err = parseSPBlock(p.r)
		if err != nil {
			return err
		}
		// reset the byte counter to prevent overflow
		defer p.r.Reset()
	default:
		return errors.New("unknown serialization type: " + st.name)
	}

	return endObject(p.r)
}
