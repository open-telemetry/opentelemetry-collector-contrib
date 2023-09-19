// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/tcp"

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"go.opentelemetry.io/collector/config/configtls"
	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	operatorType = "tcp_input"

	// minMaxLogSize is the minimal size which can be used for buffering
	// TCP input
	minMaxLogSize = 64 * 1024

	// DefaultMaxLogSize is the max buffer sized used
	// if MaxLogSize is not set
	DefaultMaxLogSize = 1024 * 1024
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new TCP input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new TCP input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
		BaseConfig: BaseConfig{
			OneLogPerPacket: false,
			Encoding:        "utf-8",
		},
	}
}

// Config is the configuration of a tcp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

// BaseConfig is the detailed configuration of a tcp input operator.
type BaseConfig struct {
	MaxLogSize       helper.ByteSize             `mapstructure:"max_log_size,omitempty"`
	ListenAddress    string                      `mapstructure:"listen_address,omitempty"`
	TLS              *configtls.TLSServerSetting `mapstructure:"tls,omitempty"`
	AddAttributes    bool                        `mapstructure:"add_attributes,omitempty"`
	OneLogPerPacket  bool                        `mapstructure:"one_log_per_packet,omitempty"`
	Encoding         string                      `mapstructure:"encoding,omitempty"`
	SplitConfig      split.Config                `mapstructure:"multiline,omitempty"`
	TrimConfig       trim.Config                 `mapstructure:",squash"`
	SplitFuncBuilder SplitFuncBuilder
}

type SplitFuncBuilder func(enc encoding.Encoding) (bufio.SplitFunc, error)

func (c Config) defaultSplitFuncBuilder(enc encoding.Encoding) (bufio.SplitFunc, error) {
	return c.SplitConfig.Func(enc, true, int(c.MaxLogSize))
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

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}

	if c.SplitFuncBuilder == nil {
		c.SplitFuncBuilder = c.defaultSplitFuncBuilder
	}

	// Build split func
	splitFunc, err := c.SplitFuncBuilder(enc)
	if err != nil {
		return nil, err
	}
	splitFunc = trim.WithFunc(splitFunc, c.TrimConfig.Func())

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	tcpInput := &Input{
		InputOperator:   inputOperator,
		address:         c.ListenAddress,
		MaxLogSize:      int(c.MaxLogSize),
		addAttributes:   c.AddAttributes,
		OneLogPerPacket: c.OneLogPerPacket,
		encoding:        enc,
		splitFunc:       splitFunc,
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
	address         string
	MaxLogSize      int
	addAttributes   bool
	OneLogPerPacket bool

	listener net.Listener
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	tls      *tls.Config
	backoff  backoff.Backoff

	encoding  encoding.Encoding
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

		dec := decode.New(t.encoding)
		if t.OneLogPerPacket {
			var buf bytes.Buffer
			_, err := io.Copy(&buf, conn)
			if err != nil {
				t.Errorw("IO copy net connection buffer error", zap.Error(err))
			}
			log := truncateMaxLog(buf.Bytes(), t.MaxLogSize)
			t.handleMessage(ctx, conn, dec, log)
			return
		}

		buf := make([]byte, 0, t.MaxLogSize)

		scanner := bufio.NewScanner(conn)
		scanner.Buffer(buf, t.MaxLogSize)

		scanner.Split(t.splitFunc)

		for scanner.Scan() {
			t.handleMessage(ctx, conn, dec, scanner.Bytes())
		}

		if err := scanner.Err(); err != nil {
			t.Errorw("Scanner error", zap.Error(err))
		}
	}()
}

func (t *Input) handleMessage(ctx context.Context, conn net.Conn, dec *decode.Decoder, log []byte) {
	decoded, err := dec.Decode(log)
	if err != nil {
		t.Errorw("Failed to decode data", zap.Error(err))
		return
	}

	entry, err := t.NewEntry(string(decoded))
	if err != nil {
		t.Errorw("Failed to create entry", zap.Error(err))
		return
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

func truncateMaxLog(data []byte, maxLogSize int) (token []byte) {
	if len(data) >= maxLogSize {
		return data[:maxLogSize]
	}

	if len(data) == 0 {
		return nil
	}

	return data
}

// Stop will stop listening for log entries over TCP.
func (t *Input) Stop() error {
	if t.cancel == nil {
		return nil
	}
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
