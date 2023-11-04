// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ipfixlookupprocessor

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/metadata"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

// ---------------
//  Rand Mock
// ---------------

var counter int

func randReadMock(b []byte) (n int, err error) {
	for i := range b {
		b[i] = byte(counter)

	}
	counter++
	return len(b), nil
}

// ---------------
//  Elastic Mock
// ---------------

func searchElasticMock(query string, _ *elasticsearch.Client) (*esapi.Response, error) {
	// find file by IP and port
	sourceIP := gjson.Get(query, "query.bool.must.#.match.source\\.ip").Array()[0].String()
	sourcePort := gjson.Get(query, "query.bool.must.#.match.source\\.port").Array()[0].String()
	destinationIP := gjson.Get(query, "query.bool.must.#.match.destination\\.ip").Array()[0].String()
	destinationPort := gjson.Get(query, "query.bool.must.#.match.destination\\.port").Array()[0].String()

	fileName := sourceIP + ":" + sourcePort + "-" + destinationIP + ":" + destinationPort + ".elastic.json"
	var body []byte
	var err error

	if fileExists(filepath.Join("testdata", "traces", fileName)) {
		body, err = readJSONFile(filepath.Join("testdata", "traces", fileName))
	} else {
		body, err = readJSONFile(filepath.Join("testdata", "traces", "empty.elastic.json"))
	}
	if err != nil {
		return &esapi.Response{
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("Elastic Search failure")),
		}, err
	}

	return &esapi.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}, nil
}

func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// readJSONFile reads the contents of a JSON file
func readJSONFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------
//  Tests
// ---------------

func TestProcessor(t *testing.T) {
	testCases := []struct {
		name string
	}{
		{"NoResult"},
		{"NoResult-int"},
		{"OneSpanOneResult"},
		{"TwoSpanOneResult"},
		{"OneSpanWithAnswer"},
		{"OneSpanWithTwoFirewalls"},
		// Add more test cases here
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create Processor
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			p := newIPFIXLookupProcessor(zaptest.NewLogger(t), cfg)

			// Fake consumer
			sink := &consumertest.TracesSink{}
			p.tracesConsumer = sink

			// Create Context
			ctx := metadata.NewIncomingContext(context.Background(), nil)
			require.NoError(t, p.Start(ctx, componenttest.NewNopHost()))
			defer func() { require.NoError(t, p.Shutdown(ctx)) }()

			// Mock rand.read Is there a better way of doing this ?
			counter = 1
			randRead = randReadMock

			// Mock elastic search lookup
			searchElasticFunc = searchElasticMock

			// traces
			tracesBefore, err := golden.ReadTraces(filepath.Join("testdata", "traces", tc.name+".before.yaml"))
			require.NoError(t, err)
			tracesAfter, err := golden.ReadTraces(filepath.Join("testdata", "traces", tc.name+".after.yaml"))
			assert.NoError(t, err)
			errConsumeTraces := p.ConsumeTraces(ctx, tracesBefore)
			assert.NoError(t, errConsumeTraces)
			sortTraceAttributes(tracesBefore) // The Traces need to be sorted as the Attribute maps can variy in order

			// golden.WriteTraces(t, filepath.Join("testdata", "traces", tc.name+".output.yaml"), sink.AllTraces()[0])

			require.Equal(t, tracesAfter, sink.AllTraces()[0])
		})
	}
}

func TestCapabilitiesMutatesData(t *testing.T) {
	// Create an instance of processorImp
	c := &processorImp{} // Assuming processorImp is the type you provided

	// Call the Capabilities method
	capabilities := c.Capabilities()

	// Check if MutatesData is true
	if !capabilities.MutatesData {
		t.Errorf("Expected MutatesData to be true, but got false")
	}
}
