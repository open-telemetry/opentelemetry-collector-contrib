// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package filelogexporter

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
)

func TestFileLogsExporter(t *testing.T) {
	// Prepare exporter
	fileNameAttributeKey := "file.path"
	basedirAttributeKey := "base.path"
	tmpDirName := tempDirName(t)
	defer func() {
		require.NoError(t, os.RemoveAll(tmpDirName))
	}()
	fe := &fileLogExporter{
		path:                 tmpDirName,
		logger:               zaptest.NewLogger(t),
		filenameAttributeKey: fileNameAttributeKey,
		basedirAttributeKey:  basedirAttributeKey,
	}

	// Prepare logs
	ld := testdata.GenerateLogsTwoLogRecordsSameResource()
	basePath := "/base/path/"
	file1 := "/log/foo.json"
	basePathAlt := "/base/path/alt"
	file2 := "/log/foo2.json"
	ld.ResourceLogs().At(0).Resource().Attributes().InsertString(basedirAttributeKey, basePath)
	ld.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().InsertString(fileNameAttributeKey, file1)
	ld.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().InsertString(basedirAttributeKey, basePathAlt)
	ld.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(1).Attributes().InsertString(fileNameAttributeKey, file2)

	// Export logs
	assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, fe.ConsumeLogs(context.Background(), ld))
	assert.NoError(t, fe.Shutdown(context.Background()))

	// Assert file content
	bufFile1, err := ioutil.ReadFile(filepath.Join(tmpDirName, basePathAlt, file1))
	assert.NoError(t, err)
	assert.EqualValues(t, "This is a log message\n", string(bufFile1))
	bufFile2, err := ioutil.ReadFile(filepath.Join(tmpDirName, basePath, file2))
	assert.NoError(t, err)
	assert.EqualValues(t, "something happened\n", string(bufFile2))
}

func TestFileLogsNoFilenameKey(t *testing.T) {
	_ = &errorWriter{}
	var message string
	zapOption := zap.Hooks(func(entry zapcore.Entry) error {
		message = entry.Message
		return nil
	})
	fe := &fileLogExporter{
		logger:               zaptest.NewLogger(t, zaptest.WrapOptions(zapOption)),
		filenameAttributeKey: "file.name",
	}
	require.NotNil(t, fe)

	ld := testdata.GenerateLogsOneLogRecord()
	assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
	assert.NoError(t, fe.ConsumeLogs(context.Background(), ld))
	assert.EqualValues(t, "file name attribute 'file.name' not found", message)
	assert.NoError(t, fe.Shutdown(context.Background()))
}

func TestFileLogsExporterErrors(t *testing.T) {
	filenameAttributeKey := "file.name"
	tmpDirName := tempDirName(t)
	defer func() {
		require.NoError(t, os.RemoveAll(tmpDirName))
	}()
	fe := &fileLogExporter{
		path:                 tmpDirName,
		logger:               zaptest.NewLogger(t),
		filenameAttributeKey: filenameAttributeKey,
	}

	ld := testdata.GenerateLogsOneLogRecord()
	fileName := "filename"
	ld.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().InsertString(filenameAttributeKey, fileName)

	assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
	fe.files.Add(filepath.Join(tmpDirName, fileName), &errorWriter{})
	assert.Error(t, fe.ConsumeLogs(context.Background(), ld))
	fe.files.Add(filepath.Join(tmpDirName, fileName), &errorWriterNewLine{})
	assert.Error(t, fe.ConsumeLogs(context.Background(), ld))
	assert.NoError(t, fe.Shutdown(context.Background()))
}

func TestFileLogsExporterErrorCreateDirAndFile(t *testing.T) {
	// Prepare exporter
	fileNameAttributeKey := "file.path"
	basedirAttributeKey := "base.path"
	tmpDirName := tempDirName(t)
	assert.NoError(t, os.Chmod(tmpDirName, 0400))
	defer func() {
		require.NoError(t, os.RemoveAll(tmpDirName))
	}()
	fe := &fileLogExporter{
		path:                 tmpDirName,
		logger:               zaptest.NewLogger(t),
		filenameAttributeKey: fileNameAttributeKey,
		basedirAttributeKey:  basedirAttributeKey,
	}

	// Prepare logs
	log1 := testdata.GenerateLogsOneLogRecord()
	log2 := testdata.GenerateLogsOneLogRecord()
	file := "file.json"
	basePath := "/base/path/"

	// Will fail on base dir creation
	log1.ResourceLogs().At(0).Resource().Attributes().InsertString(basedirAttributeKey, basePath)
	log1.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().InsertString(fileNameAttributeKey, file)

	// Will fail on log file creation
	log2.ResourceLogs().At(0).InstrumentationLibraryLogs().At(0).Logs().At(0).Attributes().InsertString(fileNameAttributeKey, file)

	// Export logs with errors
	assert.NoError(t, fe.Start(context.Background(), componenttest.NewNopHost()))
	assert.Error(t, fe.ConsumeLogs(context.Background(), log1))
	assert.Error(t, fe.ConsumeLogs(context.Background(), log2))
	assert.NoError(t, fe.Shutdown(context.Background()))
}

// tempFileName provides a temporary file name for testing.
func tempDirName(t *testing.T) string {
	tmpDir, err := ioutil.TempDir("", "exporter_test")
	require.NoError(t, err)
	return tmpDir
}

// errorWriter is an io.Writer that will return an error all ways
type errorWriter struct {
}

func (e errorWriter) Write([]byte) (n int, err error) {
	return 0, errors.New("all ways return error")
}

func (e *errorWriter) Close() error {
	return nil
}

type errorWriterNewLine struct {
}

func (e errorWriterNewLine) Write(data []byte) (n int, err error) {
	if "\n" == string(data) {
		return 0, errors.New("all ways return error")
	}
	return len(data), nil
}

func (e *errorWriterNewLine) Close() error {
	return nil
}
