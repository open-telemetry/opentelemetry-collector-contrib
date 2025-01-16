// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package valkeyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/valkeyreceiver"

import (
	"context"
	"fmt"
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

	return parseRawDataMap(str)
}

func parseRawDataMap(data string) (map[string]string, error) {
	attrs := make(map[string]string)
	lines := strings.Split(data, attrDelimiter)
	for _, line := range lines {
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			return nil, fmt.Errorf("could not cut line %q using \":\" as delimiter", line)
		}
		attrs[key] = value
	}
	return attrs, nil
}

// close client to release connection pool.
func (c *valkeyClient) close() error {
	c.client.Close()
	return nil
}
