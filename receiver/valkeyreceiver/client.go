// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package valkeyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver"

import (
	"context"
	"strings"

	"github.com/valkey-io/valkey-go"
)

var attrDelimiter = "\r\n"

// Interface for a Valkey client. Implementation can be faked for testing.
type client interface {
	// retrieves a string of key/value pairs of valkey metadata
	retrieveInfo(context.Context) (map[string]string, error)
	// close release Valkey client connection pool
	close() error
}

// Wraps a real Valkey client, implements `client` interface.
type valkeyClient struct {
	client valkey.Client
}

var _ client = (*valkeyClient)(nil)

// Creates a new real Valkey client from the passed-in valkey.Options.
func newValkeyClient(options valkey.ClientOption) (client, error) {
	innerClient, err := valkey.NewClient(options)
	if err != nil {
		return nil, err
	}
	return &valkeyClient{innerClient}, nil
}

// Retrieve Valkey INFO. We retrieve all of the 'sections'.
func (c *valkeyClient) retrieveInfo(ctx context.Context) (map[string]string, error) {
	str, err := c.client.Do(ctx, c.client.B().Info().Build()).ToString()
	if err != nil {
		return nil, err
	}

	return parseRawDataMap(str), nil
}

func parseRawDataMap(data string) map[string]string {
	attrs := make(map[string]string)
	lines := strings.Split(data, attrDelimiter)
	for _, line := range lines {
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		pair := strings.Split(line, ":")
		if len(pair) == 2 { // defensive, should always == 2
			attrs[pair[0]] = pair[1]
		}
	}
	return attrs
}

// close client to release connection pool.
func (c *valkeyClient) close() error {
	c.client.Close()
	return nil
}
