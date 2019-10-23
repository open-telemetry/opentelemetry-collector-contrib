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

// +build linux darwin

package namedpipe

import (
	"errors"

        "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver"
)

// Server is a noop implementation in case we are not on a Windows platform.
type Server struct{}

// New returns a new Windows Named Pipe Server.
func New(pipeName string, hnd udsreceiver.Handler) *Server {
	return nil
}

// ListenAndServe on a Windows named pipe..
func (s *Server) ListenAndServe() (err error) {
	return errors.New("named pipes only supported on Windows")
}

// Close implements io.Closer
func (s *Server) Close() error {
	return nil
}
