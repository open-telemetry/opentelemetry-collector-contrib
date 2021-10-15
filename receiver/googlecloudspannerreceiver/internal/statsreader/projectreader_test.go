// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package statsreader

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
)

type mockCompositeReader struct {
}

func (tcr mockCompositeReader) Name() string {
	return "mockCompositeReader"
}

func (tcr mockCompositeReader) Read(_ context.Context) []pdata.Metrics {
	return []pdata.Metrics{pdata.NewMetrics()}
}

func (tcr mockCompositeReader) Shutdown() {
	// Do nothing
}

func TestNewProjectReader(t *testing.T) {
	logger := zap.NewNop()
	var databaseReaders []CompositeReader

	reader := NewProjectReader(databaseReaders, logger)
	defer reader.Shutdown()

	assert.NotNil(t, reader)
	assert.Equal(t, logger, reader.logger)
	assert.Equal(t, databaseReaders, reader.databaseReaders)
}

func TestProjectReader_Shutdown(t *testing.T) {
	logger := zap.NewNop()

	databaseReaders := []CompositeReader{mockCompositeReader{}}

	reader := ProjectReader{
		databaseReaders: databaseReaders,
		logger:          logger,
	}

	reader.Shutdown()
}

func TestProjectReader_Read(t *testing.T) {
	ctx := context.Background()
	logger := zap.NewNop()

	databaseReaders := []CompositeReader{mockCompositeReader{}}

	reader := ProjectReader{
		databaseReaders: databaseReaders,
		logger:          logger,
	}
	defer reader.Shutdown()

	metrics := reader.Read(ctx)

	assert.Equal(t, 1, len(metrics))
}

func TestProjectReader_Name(t *testing.T) {
	logger := zap.NewNop()

	databaseReader := mockCompositeReader{}
	databaseReaders := []CompositeReader{databaseReader}

	reader := ProjectReader{
		databaseReaders: databaseReaders,
		logger:          logger,
	}
	defer reader.Shutdown()

	name := reader.Name()

	assert.Equal(t, "Project reader for: "+databaseReader.Name(), name)
}
