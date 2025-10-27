// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type PickleConfig struct{}

type PickleParser struct {
	cfg *PickleConfig
}

func NewPickleParser(cfg *PickleConfig) (Parser, error) {
	if cfg == nil {
		cfg = &PickleConfig{}
	}
	return &PickleParser{cfg: cfg}, nil
}

func (*PickleConfig) BuildParser() (Parser, error) {
	return NewPickleParser(nil)
}

func (*PickleParser) Parse(data []byte) (pmetric.Metrics, error) {
	metrics := pmetric.NewMetrics()
	// TODO
	fmt.Println("ERROR: Pickle not supported yet")
	return metrics, nil
}

func pickleDefaultConfig() ParserConfig {
	return &PickleConfig{}
}
