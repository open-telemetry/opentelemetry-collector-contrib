// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package udp // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/udp"

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"

	"go.uber.org/zap"
	"golang.org/x/text/encoding"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/decode"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/split"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/trim"
)

const (
	operatorType = "udp_input"

	// Maximum UDP packet size
	MaxUDPSize = 64 * 1024
)

func init() {
	operator.Register(operatorType, func() operator.Builder { return NewConfig() })
}

// NewConfig creates a new UDP input config with default values
func NewConfig() *Config {
	return NewConfigWithID(operatorType)
}

// NewConfigWithID creates a new UDP input config with default values
func NewConfigWithID(operatorID string) *Config {
	return &Config{
		InputConfig: helper.NewInputConfig(operatorID, operatorType),
		BaseConfig: BaseConfig{
			Encoding:        "utf-8",
			OneLogPerPacket: false,
			Multiline: split.MultilineConfig{
				LineStartPattern: "",
				LineEndPattern:   ".^", // Use never matching regex to not split data by default
			},
		},
	}
}

// Config is the configuration of a udp input operator.
type Config struct {
	helper.InputConfig `mapstructure:",squash"`
	BaseConfig         `mapstructure:",squash"`
}

// BaseConfig is the details configuration of a udp input operator.
type BaseConfig struct {
	ListenAddress   string                `mapstructure:"listen_address,omitempty"`
	OneLogPerPacket bool                  `mapstructure:"one_log_per_packet,omitempty"`
	AddAttributes   bool                  `mapstructure:"add_attributes,omitempty"`
	Encoding        string                `mapstructure:"encoding,omitempty"`
	Multiline       split.MultilineConfig `mapstructure:"multiline,omitempty"`
	TrimConfig      trim.Config           `mapstructure:",squash"`
}

// Build will build a udp input operator.
func (c Config) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	address, err := net.ResolveUDPAddr("udp", c.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %w", err)
	}

	enc, err := decode.LookupEncoding(c.Encoding)
	if err != nil {
		return nil, err
	}

	// Build multiline
	trimFunc := c.TrimConfig.Func()
	splitFunc, err := c.Multiline.Build(enc, true, MaxUDPSize, trimFunc)
	if err != nil {
		return nil, err
	}

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	udpInput := &Input{
		InputOperator:   inputOperator,
		address:         address,
		buffer:          make([]byte, MaxUDPSize),
		addAttributes:   c.AddAttributes,
		encoding:        enc,
		splitFunc:       splitFunc,
		resolver:        resolver,
		OneLogPerPacket: c.OneLogPerPacket,
	}
	return udpInput, nil
}

// Input is an operator that listens to a socket for log entries.
type Input struct {
	buffer []byte
	helper.InputOperator
	address         *net.UDPAddr
	addAttributes   bool
	OneLogPerPacket bool

	connection net.PacketConn
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	encoding  encoding.Encoding
	splitFunc bufio.SplitFunc
	resolver  *helper.IPResolver
}

// Start will start listening for messages on a socket.
func (u *Input) Start(_ operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel

	conn, err := net.ListenUDP("udp", u.address)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	u.connection = conn

	u.goHandleMessages(ctx)
	return nil
}

// goHandleMessages will handle messages from a udp connection.
func (u *Input) goHandleMessages(ctx context.Context) {
	u.wg.Add(1)

	go func() {
		defer u.wg.Done()

		dec := decode.New(u.encoding)
		buf := make([]byte, 0, MaxUDPSize)
		for {
			message, remoteAddr, err := u.readMessage()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					u.Errorw("Failed reading messages", zap.Error(err))
				}
				break
			}

			if u.OneLogPerPacket {
				log := truncateMaxLog(message)
				u.handleMessage(ctx, remoteAddr, dec, log)
				continue
			}

			scanner := bufio.NewScanner(bytes.NewReader(message))
			scanner.Buffer(buf, MaxUDPSize)

			scanner.Split(u.splitFunc)

			for scanner.Scan() {
				u.handleMessage(ctx, remoteAddr, dec, scanner.Bytes())
			}
			if err := scanner.Err(); err != nil {
				u.Errorw("Scanner error", zap.Error(err))
			}
		}
	}()
}

func truncateMaxLog(data []byte) (token []byte) {
	if len(data) >= MaxUDPSize {
		return data[:MaxUDPSize]
	}

	if len(data) == 0 {
		return nil
	}

	return data
}

func (u *Input) handleMessage(ctx context.Context, remoteAddr net.Addr, dec *decode.Decoder, log []byte) {
	decoded, err := dec.Decode(log)
	if err != nil {
		u.Errorw("Failed to decode data", zap.Error(err))
		return
	}

	entry, err := u.NewEntry(string(decoded))
	if err != nil {
		u.Errorw("Failed to create entry", zap.Error(err))
		return
	}

	if u.addAttributes {
		entry.AddAttribute("net.transport", "IP.UDP")
		if addr, ok := u.connection.LocalAddr().(*net.UDPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.host.ip", addr.IP.String())
			entry.AddAttribute("net.host.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.host.name", u.resolver.GetHostFromIP(ip))
		}

		if addr, ok := remoteAddr.(*net.UDPAddr); ok {
			ip := addr.IP.String()
			entry.AddAttribute("net.peer.ip", ip)
			entry.AddAttribute("net.peer.port", strconv.FormatInt(int64(addr.Port), 10))
			entry.AddAttribute("net.peer.name", u.resolver.GetHostFromIP(ip))
		}
	}

	u.Write(ctx, entry)
}

// readMessage will read log messages from the connection.
func (u *Input) readMessage() ([]byte, net.Addr, error) {
	n, addr, err := u.connection.ReadFrom(u.buffer)
	if err != nil {
		return nil, nil, err
	}

	// Remove trailing characters and NULs
	for ; (n > 0) && (u.buffer[n-1] < 32); n-- { // nolint
	}

	return u.buffer[:n], addr, nil
}

// Stop will stop listening for udp messages.
func (u *Input) Stop() error {
	if u.cancel == nil {
		return nil
	}
	u.cancel()
	if u.connection != nil {
		if err := u.connection.Close(); err != nil {
			u.Errorf("failed to close UDP connection: %s", err)
		}
	}
	u.wg.Wait()
	if u.resolver != nil {
		u.resolver.Stop()
	}
	return nil
}
