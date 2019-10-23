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

package unixsocket

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udsreceiver"
)

var errAlreadyClosed = errors.New("already closed")

// Server is our UnixSocket server.
type Server struct {
	addr      *net.UnixAddr
	ln        *net.UnixListener
	hnd       udsreceiver.Handler
	closeOnce int32
}

// New returns a new Unix Socket Server.
func New(socketPath string, hnd udsreceiver.Handler) *Server {
	return &Server{
		addr: &net.UnixAddr{
			Name: socketPath,
			Net:  "unix",
		},
		hnd: hnd,
	}
}

// ListenAndServe on a Unix Socket.
func (s *Server) ListenAndServe() (err error) {
	s.ln, err = net.ListenUnix("unix", s.addr)
	if err != nil {
		return err
	}
	for {
		c, err := s.ln.AcceptUnix()
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
