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

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/helper"
)

const (
	// Maximum UDP packet size
	MaxUDPSize = 64 * 1024
)

func init() {
	operator.Register("udp_input", func() operator.Builder { return NewUDPInputConfig("") })
}

// NewUDPInputConfig creates a new UDP input config with default values
func NewUDPInputConfig(operatorID string) *UDPInputConfig {
	return &UDPInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "udp_input"),
		UDPBaseConfig: UDPBaseConfig{
			Encoding: helper.NewEncodingConfig(),
			Multiline: helper.MultilineConfig{
				LineStartPattern: "",
				LineEndPattern:   ".^", // Use never matching regex to not split data by default
			},
		},
	}
}

// UDPInputConfig is the configuration of a udp input operator.
type UDPInputConfig struct {
	helper.InputConfig `yaml:",inline"`
	UDPBaseConfig      `yaml:",inline"`
}

// UDPBaseConfig is the details configuration of a udp input operator.
type UDPBaseConfig struct {
	ListenAddress string                 `mapstructure:"listen_address,omitempty"        json:"listen_address,omitempty"       yaml:"listen_address,omitempty"`
	AddAttributes bool                   `mapstructure:"add_attributes,omitempty"        json:"add_attributes,omitempty"       yaml:"add_attributes,omitempty"`
	Encoding      helper.EncodingConfig  `mapstructure:",squash,omitempty"               json:",inline,omitempty"              yaml:",inline,omitempty"`
	Multiline     helper.MultilineConfig `mapstructure:"multiline,omitempty"             json:"multiline,omitempty"            yaml:"multiline,omitempty"`
}

// Build will build a udp input operator.
func (c UDPInputConfig) Build(logger *zap.SugaredLogger) (operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(logger)
	if err != nil {
		return nil, err
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("missing required parameter 'listen_address'")
	}

	address, err := net.ResolveUDPAddr("udp", c.ListenAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve listen_address: %s", err)
	}

	encoding, err := c.Encoding.Build()
	if err != nil {
		return nil, err
	}

	// Build multiline
	splitFunc, err := c.Multiline.Build(encoding.Encoding, true, nil, MaxUDPSize)
	if err != nil {
		return nil, err
	}

	var resolver *helper.IPResolver
	if c.AddAttributes {
		resolver = helper.NewIPResolver()
	}

	udpInput := &UDPInput{
		InputOperator: inputOperator,
		address:       address,
		buffer:        make([]byte, MaxUDPSize),
		addAttributes: c.AddAttributes,
		encoding:      encoding,
		splitFunc:     splitFunc,
		resolver:      resolver,
	}
	return udpInput, nil
}

// UDPInput is an operator that listens to a socket for log entries.
type UDPInput struct {
	buffer []byte
	helper.InputOperator
	address       *net.UDPAddr
	addAttributes bool

	connection net.PacketConn
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	encoding  helper.Encoding
	splitFunc bufio.SplitFunc
	resolver  *helper.IPResolver
}

// Start will start listening for messages on a socket.
func (u *UDPInput) Start(persister operator.Persister) error {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel

	conn, err := net.ListenUDP("udp", u.address)
	if err != nil {
		return fmt.Errorf("failed to open connection: %s", err)
	}
	u.connection = conn

	u.goHandleMessages(ctx)
	return nil
}

// goHandleMessages will handle messages from a udp connection.
func (u *UDPInput) goHandleMessages(ctx context.Context) {
	u.wg.Add(1)

	go func() {
		defer u.wg.Done()

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

			scanner := bufio.NewScanner(bytes.NewReader(message))
			scanner.Buffer(buf, MaxUDPSize)

			scanner.Split(u.splitFunc)

			for scanner.Scan() {
				decoded, err := u.encoding.Decode(scanner.Bytes())
				if err != nil {
					u.Errorw("Failed to decode data", zap.Error(err))
					continue
				}

				entry, err := u.NewEntry(decoded)
				if err != nil {
					u.Errorw("Failed to create entry", zap.Error(err))
					continue
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
			if err := scanner.Err(); err != nil {
				u.Errorw("Scanner error", zap.Error(err))
			}
		}
	}()
}

// readMessage will read log messages from the connection.
func (u *UDPInput) readMessage() ([]byte, net.Addr, error) {
	n, addr, err := u.connection.ReadFrom(u.buffer)
	if err != nil {
		return nil, nil, err
	}

	// Remove trailing characters and NULs
	for ; (n > 0) && (u.buffer[n-1] < 32); n-- {
	}

	return u.buffer[:n], addr, nil
}

// Stop will stop listening for udp messages.
func (u *UDPInput) Stop() error {
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
