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

// +build windows

package namedpipe

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/Microsoft/go-winio"

        "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver"
)

const bufSize = 65536

var errAlreadyClosed = errors.New("already closed")

type Server struct {
	pipeName  string
	ln        net.Listener
	hnd       udsreceiver.Handler
	closeOnce int32
}

// New returns a new Windows Named Pipe Server.
func New(pipeName string, hnd udsreceiver.Handler) *Server {
	return &Server{
		pipeName: pipeName,
		hnd:      hnd,
	}
}

// ListenAndServe on a Windows named pipe..
func (s *Server) ListenAndServe() (err error) {
	config := &winio.PipeConfig{
		InputBufferSize:  bufSize,
		OutputBufferSize: bufSize,
	}
	s.ln, err = winio.ListenPipe(s.pipeName, config)
	if err != nil {
		return err
	}
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return err
		}
		go s.hnd.Handle(c)
	}
}

// Close implements io.Closer
func (s *Server) Close() error {
	if atomic.CompareAndSwapInt32(&s.closeOnce, 0, 1) {
		return s.ln.Close()
	}
	return errAlreadyClosed
}
