// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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

const protocolRFC5424Str = "rfc5424"
const protocolRFC3164Str = "rfc3164"

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
	switch s.protocol {
	case protocolRFC3164Str:
		return s.formatRFC3164(msg, timestamp)
	case protocolRFC5424Str:
		return s.formatRFC5424(msg, timestamp)
	default:
		panic(fmt.Sprintf("unsupported syslog protocol, protocol: %s", s.protocol))
	}
}

func (s *sender) addStructuredData(msg map[string]any) {
	if s.protocol != protocolRFC5424Str {
		return
	}

	switch sd := msg[structuredData].(type) {
	case map[string]map[string]string:
		sdElements := []string{}
		for key, val := range sd {
			sdElements = append(sdElements, key)
			for k, v := range val {
				sdElements = append(sdElements, fmt.Sprintf("%s=\"%s\"", k, v))
			}
		}
		msg[structuredData] = sdElements
	case map[string]interface{}:
		sdElements := []string{}
		for key, val := range sd {
			sdElements = append(sdElements, key)
			vval, ok := val.(map[string]interface{})
			if !ok {
				continue
			}
			for k, v := range vval {
				vv, ok := v.(string)
				if !ok {
					continue
				}
				sdElements = append(sdElements, fmt.Sprintf("%s=\"%s\"", k, vv))
			}
		}
		msg[structuredData] = sdElements
	default:
		msg[structuredData] = emptyValue
	}
}

func populateDefaults(msg map[string]any, msgProperties []string) {
	for _, msgProperty := range msgProperties {
		if _, ok := msg[msgProperty]; ok {
			continue
		}

		switch msgProperty {
		case priority:
			msg[msgProperty] = defaultPriority
		case version:
			msg[msgProperty] = versionRFC5424
		case facility:
			msg[msgProperty] = defaultFacility
		case message:
			msg[msgProperty] = emptyMessage
		default:
			msg[msgProperty] = emptyValue
		}
	}
}

func (s *sender) formatRFC3164(msg map[string]any, timestamp time.Time) string {
	msgProperties := []string{priority, hostname, message}
	populateDefaults(msg, msgProperties)
	timestampString := timestamp.Format("Jan 02 15:04:05")
	return fmt.Sprintf("<%d>%s %s%s", msg[priority], timestampString, msg[hostname], formatMessagePart(msg[message]))
}

func (s *sender) formatRFC5424(msg map[string]any, timestamp time.Time) string {
	msgProperties := []string{priority, version, hostname, app, pid, msgID, message, structuredData}
	populateDefaults(msg, msgProperties)
	s.addStructuredData(msg)
	timestampString := timestamp.Format(time.RFC3339)

	return fmt.Sprintf("<%d>%d %s %s %s %s %s %s%s", msg[priority], msg[version], timestampString, msg[hostname], msg[app], msg[pid], msg[msgID], msg[structuredData], formatMessagePart(msg[message]))
}

func formatMessagePart(message any) string {
	msg := message.(string)
	if msg != emptyMessage {
		msg = " " + msg
	}

	return msg
}
