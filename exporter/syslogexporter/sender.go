// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"

	"go.uber.org/zap"
)

const defaultPriority = 165
const versionRFC5424 = 1

const protocolRFC5424Str = "rfc5424"
const protocolRFC3164Str = "rfc3164"

const priority = "priority"
const version = "version"
const hostname = "hostname"
const app = "appname"
const pid = "proc_id"
const msgID = "msg_id"
const structuredData = "structured_data"
const message = "message"

const emptyValue = "-"
const emptyMessage = ""

type sender struct {
	network   string
	addr      string
	protocol  string
	tlsConfig *tls.Config
	logger    *zap.Logger
	mu        sync.Mutex
	conn      net.Conn
}

func connect(logger *zap.Logger, cfg *Config, tlsConfig *tls.Config) (*sender, error) {
	s := &sender{
		logger:    logger,
		network:   cfg.Network,
		addr:      fmt.Sprintf("%s:%d", cfg.Endpoint, cfg.Port),
		protocol:  cfg.Protocol,
		tlsConfig: tlsConfig,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.dial()
	if err != nil {
		return nil, err
	}
	return s, err
}

func (s *sender) close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

func (s *sender) dial() error {
	if s.conn != nil {
		s.conn.Close()
		s.conn = nil
	}
	var err error
	if s.tlsConfig != nil {
		s.conn, err = tls.Dial("tcp", s.addr, s.tlsConfig)
	} else {
		s.conn, err = net.Dial(s.network, s.addr)
	}
	return err
}

func (s *sender) Write(msgStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn != nil {
		if err := s.write(msgStr); err == nil {
			return nil
		}
	}
	if err := s.dial(); err != nil {
		return err
	}

	return s.write(msgStr)
}
func (s *sender) write(msg string) error {
	// check if logs contains new line character at the end, if not add it
	if !strings.HasSuffix(msg, "\n") {
		msg = fmt.Sprintf("%s%s", msg, "\n")
	}
	_, err := fmt.Fprint(s.conn, msg)
	return err
}
