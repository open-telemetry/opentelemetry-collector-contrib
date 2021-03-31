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

package udp

import (
	"context"
	"fmt"
	"net"
	"sync"

	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-log-collection/operator"
	"github.com/open-telemetry/opentelemetry-log-collection/operator/helper"
)

func init() {
	operator.Register("udp_input", func() operator.Builder { return NewUDPInputConfig("") })
}

// NewUDPInputConfig creates a new UDP input config with default values
func NewUDPInputConfig(operatorID string) *UDPInputConfig {
	return &UDPInputConfig{
		InputConfig: helper.NewInputConfig(operatorID, "udp_input"),
	}
}

// UDPInputConfig is the configuration of a udp input operator.
type UDPInputConfig struct {
	helper.InputConfig `yaml:",inline"`

	ListenAddress string `json:"listen_address,omitempty" yaml:"listen_address,omitempty"`
}

// Build will build a udp input operator.
func (c UDPInputConfig) Build(context operator.BuildContext) ([]operator.Operator, error) {
	inputOperator, err := c.InputConfig.Build(context)
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

	udpInput := &UDPInput{
		InputOperator: inputOperator,
		address:       address,
		buffer:        make([]byte, 8192),
	}
	return []operator.Operator{udpInput}, nil
}

// UDPInput is an operator that listens to a socket for log entries.
type UDPInput struct {
	buffer []byte
	helper.InputOperator
	address *net.UDPAddr

	connection net.PacketConn
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// Start will start listening for messages on a socket.
func (u *UDPInput) Start() error {
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

		for {
			message, err := u.readMessage()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					u.Errorw("Failed reading messages", zap.Error(err))
				}
				break
			}

			entry, err := u.NewEntry(message)
			if err != nil {
				u.Errorw("Failed to create entry", zap.Error(err))
				continue
			}

			u.Write(ctx, entry)
		}
	}()
}

// readMessage will read log messages from the connection.
func (u *UDPInput) readMessage() (string, error) {
	n, _, err := u.connection.ReadFrom(u.buffer)
	if err != nil {
		return "", err
	}

	// Remove trailing characters and NULs
	for ; (n > 0) && (u.buffer[n-1] < 32); n-- {
	}

	return string(u.buffer[:n]), nil
}

// Stop will stop listening for udp messages.
func (u *UDPInput) Stop() error {
	u.cancel()
	u.connection.Close()
	u.wg.Wait()
	return nil
}
