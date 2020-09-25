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

package requests

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClient struct {
	resp *http.Response
	err  error
}

func (f *fakeClient) Do(r *http.Request) (*http.Response, error) {
	return f.resp, f.err
}

type body struct {
	reader  io.Reader
	readErr error
}

func (b *body) Read(p []byte) (n int, err error) {
	r, _ := b.reader.Read(p)
	return r, b.readErr
}

func (b *body) Close() error {
	return nil
}

func TestReadFailed(t *testing.T) {
	called := false
	ctx := ContextWithFailed(context.Background(), func(body []byte, statusCode int, err error) {
		called = true
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	require.NoError(t, err)
	sender := &CallbackSender{Client: &fakeClient{resp: &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       &body{reader: bytes.NewBufferString("sdf"), readErr: errors.New("read failed")}},
	}}
	require.EqualError(t, sender.SendRequest(req), "error making HTTP request to http://localhost: read failed")
	assert.True(t, called)
}

func TestRequestFailedWithCallback(t *testing.T) {
	called := false
	ctx := ContextWithFailed(context.Background(), func(body []byte, statusCode int, err error) {
		called = true
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	require.NoError(t, err)
	sender := &CallbackSender{Client: &fakeClient{err: errors.New("request error")}}
	require.Error(t, sender.SendRequest(req))
	assert.True(t, called)
}

func TestRequestSucceededWithCallback(t *testing.T) {
	called := false
	ctx := ContextWithSuccess(context.Background(), func(statusCode int, body []byte) {
		called = true
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost", nil)
	require.NoError(t, err)
	sender := &CallbackSender{Client: &fakeClient{resp: &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       &body{reader: bytes.NewBufferString("OK"), readErr: io.EOF},
	}}}
	require.NoError(t, sender.SendRequest(req))
	assert.True(t, called)
}
