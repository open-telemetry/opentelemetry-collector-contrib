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

package syslogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter"

import (
	"crypto/tls"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const defaultPriority = 165
const defaultFacility = 1
const versionRFC5424 = 1

const formatRFC5424Str = "rfc5424"
const formatRFC3164Str = "rfc3164"

const priority = "priority"
const facility = "facility"
const version = "version"
const hostname = "hostname"
const app = "appname"
const pid = "proc_id"
const msgID = "msg_id"
const structuredData = "structured_data"
const message = "message"

const emptyValue = "-"

type sender struct {
	network   string
	addr      string
	format    string
	tlsConfig *tls.Config
	logger    *zap.Logger
	mu        sync.Mutex
	conn      net.Conn
}

func connect(logger *zap.Logger, cfg *Config, tlsConfig *tls.Config) (*sender, error) {
	s := &sender{
		logger:    logger,
		network:   cfg.Protocol,
		addr:      fmt.Sprintf("%s:%d", cfg.Endpoint, cfg.Port),
		format:    cfg.Format,
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

func (s *sender) Write(msg map[string]any, timestamp time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	msgStr := s.formatMsg(msg, timestamp)

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

func (s *sender) formatMsg(msg map[string]any, timestamp time.Time) string {
	switch s.format {
	case formatRFC3164Str:
		return s.formatRFC3164(msg, timestamp)
	case formatRFC5424Str:
		return s.formatRFC5424(msg, timestamp)
	default:
		panic(fmt.Sprintf("unsupported syslog format, format: %s", s.format))
	}
}

func (s *sender) addStructuredData(msg map[string]any) {
	if s.format != formatRFC5424Str {
		return
	}

	sd, ok := msg[structuredData].(map[string]map[string]string)
	if !ok {
		msg[structuredData] = emptyValue
	} else {
		sdElements := []string{}
		for key, val := range sd {
			sdElements = append(sdElements, key)
			for k, v := range val {
				sdElements = append(sdElements, fmt.Sprintf("%s=\"%s\"", k, v))
			}
		}
		msg[structuredData] = sdElements
	}
}

func populateDefaults(msg map[string]any, msgProperties []string) {

	for _, msgProperty := range msgProperties {
		msgValue, ok := msg[msgProperty]
		if !ok && msgProperty == priority {
			msg[msgProperty] = defaultPriority
			return
		}
		if !ok && msgProperty == version {
			msg[msgProperty] = versionRFC5424
			return
		}
		if !ok && msgProperty == facility {
			msg[msgProperty] = defaultFacility
			return
		}
		if !ok {
			msg[msgProperty] = emptyValue
			return
		}
		msg[msgProperty] = msgValue
	}
}

func (s *sender) formatRFC3164(msg map[string]any, timestamp time.Time) string {
	msgProperties := []string{priority, hostname, message}
	populateDefaults(msg, msgProperties)
	timestampString := timestamp.Format("2006-01-02T15:04:05.000-03:00")
	return fmt.Sprintf("<%d>%s %s %s", msg[priority], timestampString, msg[hostname], msg[message])
}

func (s *sender) formatRFC5424(msg map[string]any, timestamp time.Time) string {
	msgProperties := []string{priority, version, hostname, app, pid, msgID, message, structuredData}
	populateDefaults(msg, msgProperties)
	s.addStructuredData(msg)
	timestampString := timestamp.Format(time.RFC3339)
	return fmt.Sprintf("<%d>%d %s %s %s %s %s %s %s", msg[priority], msg[version], timestampString, msg[hostname], msg[app], msg[pid], msg[msgID], msg[structuredData], msg[message])
}
