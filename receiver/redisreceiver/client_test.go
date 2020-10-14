// Copyright 2020, OpenTelemetry Authors
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

package redisreceiver

import (
	"io/ioutil"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ client = (*fakeClient)(nil)

type fakeClient struct{}

func newFakeClient() *fakeClient {
	return &fakeClient{}
}

func (c fakeClient) delimiter() string {
	if runtime.GOOS == "windows" {
		return "\r\n"
	}

	return "\n"
}

func (fakeClient) retrieveInfo() (string, error) {
	return readFile("info")
}

func readFile(fname string) (string, error) {
	file, err := ioutil.ReadFile(path.Join("testdata", fname+".txt"))
	if err != nil {
		return "", err
	}
	return string(file), nil
}

func TestRetrieveInfo(t *testing.T) {
	g := fakeClient{}
	res, err := g.retrieveInfo()
	require.Nil(t, err)
	require.True(t, strings.HasPrefix(res, "# Server"))
}
