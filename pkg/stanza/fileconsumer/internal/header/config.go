// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package header // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/fileconsumer/internal/header"

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"regexp"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/pipeline"
)

type Config struct {
	regex             *regexp.Regexp
	SplitFunc         bufio.SplitFunc
	metadataOperators []operator.Config
}

func NewConfig(matchRegex string, metadataOperators []operator.Config, enc encoding.Encoding) (*Config, error) {
	var err error
	if len(metadataOperators) == 0 {
		return nil, errors.New("at least one operator must be specified for `metadata_operators`")
	}

	if enc == nil {
		return nil, errors.New("encoding must be specified")
	}

	nopLogger := zap.NewNop().Sugar()
	p, err := pipeline.Config{
		Operators:     metadataOperators,
		DefaultOutput: newPipelineOutput(nopLogger),
	}.Build(nopLogger)

	if err != nil {
		return nil, fmt.Errorf("failed to build pipelines: %w", err)
	}

	for _, op := range p.Operators() {
		// This is the default output we created, it's always valid
		if op.Type() == pipelineOutputType {
			continue
		}

		if !op.CanProcess() {
			return nil, fmt.Errorf("operator '%s' in `metadata_operators` cannot process entries", op.ID())
		}

		if !op.CanOutput() {
			return nil, fmt.Errorf("operator '%s' in `metadata_operators` does not propagate entries", op.ID())
		}

		// Filter processor also may fail to propagate some entries
		if op.Type() == "filter" {
			return nil, fmt.Errorf("operator of type filter is not allowed in `metadata_operators`")
		}
	}

	regex, err := regexp.Compile(matchRegex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile `pattern`: %w", err)
	}

	splitFunc, err := helper.NewNewlineSplitFunc(enc, false, func(b []byte) []byte {
		return bytes.Trim(b, "\r\n")
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create split func: %w", err)
	}

	return &Config{
		regex:             regex,
		SplitFunc:         splitFunc,
		metadataOperators: metadataOperators,
	}, nil
}
