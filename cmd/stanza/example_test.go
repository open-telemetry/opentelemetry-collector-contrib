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

package main

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	out "github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/stdout"
	"github.com/stretchr/testify/require"
)

type muxWriter struct {
	buffer bytes.Buffer
	sync.Mutex
}

func (b *muxWriter) Write(p []byte) (n int, err error) {
	b.Lock()
	defer b.Unlock()
	return b.buffer.Write(p)
}

func (b *muxWriter) String() string {
	b.Lock()
	defer b.Unlock()
	return b.buffer.String()
}

func TestTomcatExample(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on windows because of service failures")
	}
	err := os.Chdir("../../examples/tomcat")
	require.NoError(t, err)
	defer func() {
		err := os.Chdir("../../cmd/stanza")
		require.NoError(t, err)
	}()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{})

	buf := muxWriter{}
	out.Stdout = &buf

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		err = cmd.ExecuteContext(ctx)
		require.NoError(t, err)
	}()
	defer func() { <-done }()

	expected := `{"timestamp":"2019-03-13T10:43:00-04:00","severity":60,"severity_text":"404","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"-","http_method":"GET","http_status":"404","remote_host":"10.66.2.46","remote_user":"-","url_path":"/"}}
{"timestamp":"2019-03-13T10:43:01-04:00","severity":60,"severity_text":"404","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"-","http_method":"GET","http_status":"404","remote_host":"10.66.2.46","remote_user":"-","url_path":"/favicon.ico"}}
{"timestamp":"2019-03-13T10:43:08-04:00","severity":30,"severity_text":"302","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"-","http_method":"GET","http_status":"302","remote_host":"10.66.2.46","remote_user":"-","url_path":"/manager"}}
{"timestamp":"2019-03-13T10:43:08-04:00","severity":60,"severity_text":"403","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"3420","http_method":"GET","http_status":"403","remote_host":"10.66.2.46","remote_user":"-","url_path":"/manager/"}}
{"timestamp":"2019-03-13T11:00:26-04:00","severity":60,"severity_text":"401","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"2473","http_method":"GET","http_status":"401","remote_host":"10.66.2.46","remote_user":"-","url_path":"/manager/html"}}
{"timestamp":"2019-03-13T11:00:53-04:00","severity":20,"severity_text":"200","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"11936","http_method":"GET","http_status":"200","remote_host":"10.66.2.46","remote_user":"tomcat","url_path":"/manager/html"}}
{"timestamp":"2019-03-13T11:00:53-04:00","severity":20,"severity_text":"200","labels":{"file_name":"access.log","log_type":"tomcat"},"record":{"bytes_sent":"19698","http_method":"GET","http_status":"200","remote_host":"10.66.2.46","remote_user":"-","url_path":"/manager/images/asf-logo.svg"}}
`

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-time.After(100 * time.Millisecond):
			if len(strings.Split(buf.String(), "\n")) == len(strings.Split(expected, "\n")) {
				defer cancel()
				require.Equal(t, expected, buf.String())
				return
			}
		case <-timeout:
			require.FailNow(t, "Timed out waiting for logs to be written to stdout")
		}
	}
}

func TestSimplePluginsExample(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on windows because of service failures")
	}
	err := os.Chdir("../../examples/simple_plugins")
	require.NoError(t, err)
	defer func() {
		err := os.Chdir("../../cmd/stanza")
		require.NoError(t, err)
	}()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"--plugin_dir", "./plugins"})

	buf := muxWriter{}
	out.Stdout = &buf

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		err = cmd.ExecuteContext(ctx)
		require.NoError(t, err)
	}()
	defer func() { <-done }()

	expected := `{"timestamp":"2006-01-02T15:04:05Z","severity":0,"labels":{"decorated":"my_decorated_value"},"record":"test record"}
{"timestamp":"2006-01-02T15:04:05Z","severity":0,"labels":{"decorated":"my_decorated_value"},"record":"test record"}
{"timestamp":"2006-01-02T15:04:05Z","severity":0,"labels":{"decorated":"my_decorated_value"},"record":"test record"}
{"timestamp":"2006-01-02T15:04:05Z","severity":0,"labels":{"decorated":"my_decorated_value"},"record":"test record"}
{"timestamp":"2006-01-02T15:04:05Z","severity":0,"labels":{"decorated":"my_decorated_value"},"record":"test record"}
`

	timeout := time.After(5 * time.Second)
	for {
		select {
		case <-time.After(100 * time.Millisecond):
			if len(strings.Split(buf.String(), "\n")) == len(strings.Split(expected, "\n")) {
				defer cancel()
				require.Equal(t, expected, buf.String())
				return
			}
		case <-timeout:
			require.FailNow(t, "Timed out waiting for logs to be written to stdout")
		}
	}
}
