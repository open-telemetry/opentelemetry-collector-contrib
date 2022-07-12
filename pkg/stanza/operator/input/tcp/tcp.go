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

package tcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	// minMaxLogSize is the minimal size which can be used for buffering
	// TCP input
	minMaxLogSize = 64 * 1024

	// DefaultMaxLogSize is the max buffer sized used
	// if MaxLogSize is not set
	DefaultMaxLogSize = 1024 * 1024
)

func init() {
	operator.Register("tcp_input", func() operator.Builder { return NewConfig("") })
}

// NewConfig creates a new TCP input config with default values
func NewConfig(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, "tcp_input"),
		BaseConfig: BaseConfig{
			Multiline: helper.NewMultilineConfig(),
			Encoding:  helper.NewEncodingConfig(),
		},
	}
}

// Config is the configuration of a tcp input operator.
type Config struct {
	helper.InputConfig `yaml:",inline"`
	BaseConfig         `yaml:",inline"`
}

// BaseConfig is the detailed configuration of a tcp input operator.
type BaseConfig struct {
	MaxLogSize    helper.ByteSize         `mapstructure:"max_log_size,omitempty"          json:"max_log_size,omitempty"         yaml:"max_log_size,omitempty"`
	ListenAddress string                  `mapstructure:"listen_address,omitempty"        json:"listen_address,omitempty"       yaml:"listen_address,omitempty"`
	TLS           *helper.TLSServerConfig `mapstructure:"tls,omitempty"                   json:"tls,omitempty"                  yaml:"tls,omitempty"`
	AddAttributes bool                    `mapstructure:"add_attributes,omitempty"        json:"add_attributes,omitempty"       yaml:"add_attributes,omitempty"`
	Encoding      helper.EncodingConfig   `mapstructure:",squash,omitempty"               json:",inline,omitempty"              yaml:",inline,omitempty"`
	Multiline     helper.MultilineConfig  `mapstructure:"multiline,omitempty"             json:"multiline,omitempty"            yaml:"multiline,omitempty"`
}

// Build will build a tcp input operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	// If MaxLogSize not set, set sane default
	if c.MaxLogSize == 0 {
		c.MaxLogSize = DefaultMaxLogSize
	}

	if c.MaxLogSize < minMaxLogSize {
		return nil, fmt.Errorf("invalid value for parameter 'max_log_size', must be equal to or greater than %d bytes", minMaxLogSize)
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	// validate the input address
	if _, err = net.ResolveTCPAddr("tcp", c.ListenAddress); err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	encoding, err := c.Encoding.Build()
	if err != nil {
		return nil, err
	}

	// Build multiline
	splitFunc, err := c.Multiline.Build(encoding.Encoding, true, nil, int(c.MaxLogSize))
	if err != nil {
		return nil, err
	}

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	tcpInput := &Input{
		InputOperator: inputOperator,
		address:       c.ListenAddress,
		MaxLogSize:    int(c.MaxLogSize),
		addAttributes: c.AddAttributes,
		encoding:      encoding,
		splitFunc:     splitFunc,
		backoff: backoff.Backoff{
			Max: 3 * time.Second,
		},
		resolver: resolver,
	}

	if c.TLS != nil {
		tcpInput.tls, err = c.TLS.LoadTLSConfig()
		if err != nil {
			return nil, err
		}
	}

	return tcpInput, nil
}

// Input is an operator that listens for log entries over tcp.
type Input struct {
	helper.InputOperator
	address       string
	MaxLogSize    int
	addAttributes bool

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	tls      *tls.Config
	backoff  backoff.Backoff

	encoding  helper.Encoding
	splitFunc bufio.SplitFunc
	resolver  *helper.IPResolver
}

// Start will start listening for log entries over tcp.
func (t *Input) Start(_ operator.Persister) error {
	if err := t.configureListener(); err != nil {
		return fmt.Errorf("failed to listen on interface: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	t.goListen(ctx)
	return nil
}

func (t *Input) configureListener() error {
	if t.tls == nil {
		listener, err := net.Listen("tcp", t.address)
		if err != nil {
			return fmt.Errorf("failed to configure tcp listener: %w", err)
		}
		t.listener = listener
		return nil
	}

	t.tls.Time = time.Now
	t.tls.Rand = rand.Reader

	listener, err := tls.Listen("tcp", t.address, t.tls)
	if err != nil {
		return fmt.Errorf("failed to configure tls listener: %w", err)
	}

	t.listener = listener
	return nil
}

// goListenn will listen for tcp connections.
func (t *Input) goListen(ctx context.Context) {
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()

		for {
			conn, err := t.listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					t.Debugw("Listener accept error", zap.Error(err))
					time.Sleep(t.backoff.Duration())
					continue
				}
			}
			t.backoff.Reset()

			t.Debugf("Received connection: %s", conn.RemoteAddr().String())
			subctx, cancel := context.WithCancel(ctx)
			t.goHandleClose(subctx, conn)
			t.goHandleMessages(subctx, conn, cancel)
		}
	}()
}

// goHandleClose will wait for the context to finish before closing a connection.
func (t *Input) goHandleClose(ctx context.Context, conn net.Conn) {
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()
		<-ctx.Done()
		t.Debugf("Closing connection: %s", conn.RemoteAddr().String())
		if err := conn.Close(); err != nil {
			t.Errorf("Failed to close connection: %s", err)
		}
	}()
}

// goHandleMessages will handles messages from a tcp connection.
func (t *Input) goHandleMessages(ctx context.Context, conn net.Conn, cancel context.CancelFunc) {
	t.wg.Add(1)

	go func() {
		defer t.wg.Done()
		defer cancel()

		buf := make([]byte, 0, t.MaxLogSize)
		scanner := bufio.NewScanner(conn)
		scanner.Buffer(buf, t.MaxLogSize)

		scanner.Split(t.splitFunc)

		for scanner.Scan() {
			decoded, err := t.encoding.Decode(scanner.Bytes())
			if err != nil {
				t.Errorw("Failed to decode data", zap.Error(err))
				continue
			}

			entry, err := t.NewEntry(string(decoded))
			if err != nil {
				t.Errorw("Failed to create entry", zap.Error(err))
				continue
			}

			if t.addAttributes {
				entry.AddAttribute("net.transport", "IP.TCP")
				if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
					ip := addr.IP.String()
					entry.AddAttribute("net.peer.ip", ip)
					entry.AddAttribute("net.peer.port", strconv.FormatInt(int64(addr.Port), 10))
					entry.AddAttribute("net.peer.name", t.resolver.GetHostFromIP(ip))
				}

				if addr, ok := conn.LocalAddr().(*net.TCPAddr); ok {
					ip := addr.IP.String()
					entry.AddAttribute("net.host.ip", addr.IP.String())
					entry.AddAttribute("net.host.port", strconv.FormatInt(int64(addr.Port), 10))
					entry.AddAttribute("net.host.name", t.resolver.GetHostFromIP(ip))
				}
			}

			t.Write(ctx, entry)
		}
		if err := scanner.Err(); err != nil {
			t.Errorw("Scanner error", zap.Error(err))
		}
	}()
}

// Stop will stop listening for log entries over TCP.
func (t *Input) Stop() error {
	t.cancel()

	if t.listener != nil {
		if err := t.listener.Close(); err != nil {
			t.Errorf("failed to close TCP connection: %s", err)
		}
	}

	t.wg.Wait()
	if t.resolver != nil {
		t.resolver.Stop()
	}
	return nil
}
