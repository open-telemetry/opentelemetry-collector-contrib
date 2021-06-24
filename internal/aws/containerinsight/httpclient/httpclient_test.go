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

package httpClient

import (
	"bytes"
	"context"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fakeClient struct {
	response *http.Response
	err      error
}

func (f *fakeClient) Do(req *http.Request) (*http.Response, error) {
	return f.response, nil
}

func TestRequestSecuess(t *testing.T) {

	respBody := "body"
	response := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString(respBody)),
		Header:     make(http.Header),
	}

	fakeClient := &fakeClient{
		response: response,
		err:      nil,
	}

	http := New(withClientOption(fakeClient))

	ctx := context.Background()

	body, err := http.Request(ctx, "0.0.0.0", zap.NewNop())

	assert.Nil(t, err)

	assert.NotNil(t, body)

}

func TestRequestFailed(t *testing.T) {

	respBody := "body"
	response := &http.Response{
		Status:     "200 OK",
		StatusCode: 400,
		Body:       ioutil.NopCloser(bytes.NewBufferString(respBody)),
		Header:     make(http.Header),
	}

	fakeClient := &fakeClient{
		response: response,
		err:      nil,
	}

	http := New(withClientOption(fakeClient))

	ctx := context.Background()

	body, err := http.Request(ctx, "0.0.0.0", zap.NewNop())

	assert.Nil(t, body)

	assert.NotNil(t, err)

}
