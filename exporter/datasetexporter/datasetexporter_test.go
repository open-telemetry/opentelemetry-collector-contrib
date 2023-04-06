// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package datasetexporter

import (
	"os"
	"testing"

	"github.com/maxatome/go-testdeep/helpers/tdsuite"
	"github.com/maxatome/go-testdeep/td"
)

type SuiteDataSetExporter struct{}

func (s *SuiteDataSetExporter) PreTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteDataSetExporter) PostTest(t *td.T, testName string) error {
	os.Clearenv()
	return nil
}

func (s *SuiteDataSetExporter) Destroy(t *td.T) error {
	os.Clearenv()
	return nil
}

func TestSuiteDataSetExporter(t *testing.T) {
	td.NewT(t)
	tdsuite.Run(t, &SuiteDataSetExporter{})
}

func (s *SuiteDataSetExporter) TestFoo(assert, require *td.T) {
	assert.Cmp("AA", "AA")
}
