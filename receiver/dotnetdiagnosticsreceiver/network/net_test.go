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

package network

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnect(t *testing.T) {
	conn, err := Connect(1234, func(network, address string) (net.Conn, error) {
		return nil, nil
	}, testMatcher)
	assert.NoError(t, err)
	assert.Nil(t, conn)
}

func TestConnect_SocketFileErr(t *testing.T) {
	_, err := Connect(0, nil, func(pattern string) (matches []string, err error) {
		return nil, errors.New("foo")
	})
	require.EqualError(t, err, "foo")
}

func TestGlob(t *testing.T) {
	glob := globPattern(1234)
	assert.Equal(t, "dotnet-diagnostic-1234-*-socket", glob)
}

func TestSocketFile(t *testing.T) {
	socketFile, err := socketFile(1234, "/tmp", testMatcher)
	assert.NoError(t, err)
	assert.Equal(t, "/tmp/dotnet-diagnostic-1234-*-socket", socketFile)
}

func TestSocketFile_Error(t *testing.T) {
	_, err := socketFile(1234, "/tmp", testErrorMatcher)
	assert.Error(t, err)
}

func TestSocketFile_NoMatches(t *testing.T) {
	_, err := socketFile(1234, "/tmp", testNoResultsMatcher)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no matches")
}

func TestSocketFile_MultiMatches(t *testing.T) {
	_, err := socketFile(1234, "/tmp", testMultiResultsMatcher)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "multiple matches")
}

func testMatcher(pattern string) (matches []string, err error) {
	return []string{pattern}, nil
}

func testErrorMatcher(pattern string) (matches []string, err error) {
	return nil, errors.New("")
}

func testNoResultsMatcher(pattern string) (matches []string, err error) {
	return nil, nil
}

func testMultiResultsMatcher(pattern string) (matches []string, err error) {
	return []string{pattern, "foo"}, nil
}
