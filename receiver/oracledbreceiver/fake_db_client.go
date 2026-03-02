// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oracledbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver"

import (
	"context"
)

type fakeDbClient struct {
	Err            error
	Responses      [][]metricRow
	RequestCounter int
}

func (c *fakeDbClient) metricRows(context.Context, ...any) ([]metricRow, error) {
	if c.Err != nil {
		return nil, c.Err
	}
	idx := c.RequestCounter
	c.RequestCounter++
	return c.Responses[idx], nil
}
