// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datasetexporter

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type SuiteDataSetExporter struct {
	suite.Suite
}

func (s *SuiteDataSetExporter) SuiteDataSetExporter() {
	os.Clearenv()
}

func TestSuiteDataSetExporter(t *testing.T) {
	suite.Run(t, new(SuiteDataSetExporter))
}
