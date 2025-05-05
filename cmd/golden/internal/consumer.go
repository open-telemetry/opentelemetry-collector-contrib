// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/cmd/golden/internal"

import (
	"context"
	"testing"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
)

var _ consumer.Metrics = (*MetricsSink)(nil)

type MetricsSink struct {
	cfg        *Config
	noExpected bool
	expected   pmetric.Metrics
	DoneChan   chan struct{}
	Error      error
}

func (m *MetricsSink) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{
		MutatesData: false,
	}
}

func (m *MetricsSink) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	if m.noExpected {
		if m.cfg.WriteExpected {
			err := golden.WriteMetricsToFile(m.cfg.ExpectedFile, md)
			if err != nil {
				return err
			}
		}
		close(m.DoneChan)
		return nil
	}
	err := pmetrictest.CompareMetrics(m.expected, md, m.cfg.CompareOptions...)
	if err == nil {
		m.Error = nil
		if m.cfg.WriteExpected {
			err = golden.WriteMetrics(&testing.T{}, m.cfg.ExpectedFile, md)
			if err != nil {
				return err
			}
		}
		close(m.DoneChan)
	} else {
		m.Error = err
	}
	return nil
}

func NewConsumer(cfg *Config) (*MetricsSink, error) {
	expected, err := golden.ReadMetrics(cfg.ExpectedFile)
	noExpected := err != nil
	if err != nil && !cfg.WriteExpected {
		return nil, err
	}
	return &MetricsSink{
		expected:   expected,
		cfg:        cfg,
		noExpected: noExpected,
		DoneChan:   make(chan struct{}),
	}, nil
}
