// Copyright 2019, OpenTelemetry Authors
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
package util

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestSplitHeaderBodyWithSeparatorExists(t *testing.T) {
	str := "Header\nBody"
	separator := "\n"
	buf := []byte(str)
	separatorArray := []byte(separator)
	result := make([][]byte, 2)

	returnResult := SplitHeaderBody(zap.NewNop(), &buf, &separatorArray, &result)

	assert.EqualValues(t, len(result), 2)
	assert.EqualValues(t, string(result[0]), "Header")
	assert.EqualValues(t, string(result[1]), "Body")
	assert.EqualValues(t, string(returnResult[0]), "Header")
	assert.EqualValues(t, string(returnResult[1]), "Body")
	assert.EqualValues(t, string(buf), str)
	assert.EqualValues(t, string(separatorArray), separator)
}

func TestSplitHeaderBodyWithSeparatorDoesNotExist(t *testing.T) {
	str := "Header"
	separator := "\n"
	buf := []byte(str)
	separatorArray := []byte(separator)
	result := make([][]byte, 2)

	returnResult := SplitHeaderBody(zap.NewNop(), &buf, &separatorArray, &result)

	assert.EqualValues(t, len(result), 2)
	assert.EqualValues(t, string(result[0]), "Header")
	assert.EqualValues(t, string(result[1]), "")
	assert.EqualValues(t, string(returnResult[0]), "Header")
	assert.EqualValues(t, string(returnResult[1]), "")
	assert.EqualValues(t, string(buf), str)
	assert.EqualValues(t, string(separatorArray), separator)
}

func TestSplitHeaderBodyNilBuf(t *testing.T) {
	logger, recorded := logSetup()
	separator := "\n"
	separatorArray := []byte(separator)
	result := make([][]byte, 2)
	SplitHeaderBody(logger, nil, &separatorArray, &result)

	logs := recorded.All()
	assert.True(t, strings.Contains(logs[0].Message, "buffer to split is nil"))
}

func TestSplitHeaderBodyNilSeparator(t *testing.T) {
	logger, recorded := logSetup()
	str := "Test String"
	buf := []byte(str)
	result := make([][]byte, 2)

	SplitHeaderBody(logger, &buf, nil, &result)

	logs := recorded.All()
	assert.True(t, strings.Contains(logs[0].Message, "separator used to split the buffer is nil"))
}

func TestSplitHeaderBodyNilResult(t *testing.T) {
	logger, recorded := logSetup()
	str := "Test String"
	buf := []byte(str)
	separator := "\n"
	separatorArray := []byte(separator)
	SplitHeaderBody(logger, &buf, &separatorArray, nil)

	logs := recorded.All()
	assert.True(t, strings.Contains(logs[0].Message, "buffer used to store splitted result is nil"))
}

func logSetup() (*zap.Logger, *observer.ObservedLogs) {
	core, recorded := observer.New(zapcore.InfoLevel)
	return zap.New(core), recorded
}
