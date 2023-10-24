// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License. language governing permissions and
// limitations under the License.
package sflowreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sflowreceiver"

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type sflowreceiverlogs struct {
	host           component.Host
	cancel         context.CancelFunc
	nextConsumer   consumer.Logs
	config         *Config
	createSettings receiver.CreateSettings
	connection     *net.UDPConn
}

func (s *sflowreceiverlogs) Start(ctx context.Context, host component.Host) error {
	logger := s.createSettings.Logger
	translate := Translator{Logger: logger}

	ctx, s.cancel = context.WithCancel(ctx)

	// Create a UDP address to listen on.
	udpAddr, err := net.ResolveUDPAddr("udp", s.config.Endpoint)
	if err != nil {
		logger.Error("Error resolving UDP address:", zap.Error(err))
		return err
	}

	// Create a UDP connection.
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		logger.Error("Error creating UDP connection:", zap.Error(err))
		return err
	}
	s.connection = conn

	// Create a buffer to hold incoming UDP packets.
	buffer := make([]byte, 9000)

	logger.Info(fmt.Sprintf("Sflow receiver is listening on %v:%d", udpAddr.IP, udpAddr.Port))
	logger.Info(fmt.Sprintf("Labels %v", s.config.Labels))

	type udpData struct {
		size    int
		pktAddr *net.UDPAddr
		payload []byte
	}
	stopped := atomic.Value{}
	stopped.Store(false)
	udpDataCh := make(chan udpData)

	go func() {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					u := udpData{}
					u.size, u.pktAddr, err = conn.ReadFromUDP(buffer)
					if err != nil {
						if strings.Contains(err.Error(), "use of closed network connection") {
							return
						} else if err == io.EOF { // io.EOF is returned when the connection is closed.
							return
						}
						logger.Error("Error reading UDP packet:", zap.Error(err))
						continue
					}
					if stopped.Load() == false {
						if u.size == 0 { // Ignore 0 byte packets.
							continue
						}
						u.payload = make([]byte, u.size)
						copy(u.payload, buffer[0:u.size])
						udpDataCh <- u
					} else {
						return
					}
				}
			}
		}()
		for {
			select {
			case u := <-udpDataCh:
				sflowData := DecodeSFlowPacket(u.payload)
				plogs := translate.SflowToOtelLogs(sflowData, s.config)
				if plogs.LogRecordCount() > 0 {
					err := s.nextConsumer.ConsumeLogs(ctx, plogs)
					if err != nil {
						logger.Error("Error consuming logs:", zap.Error(err))
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (s *sflowreceiverlogs) Shutdown(ctx context.Context) error {
	logger := s.createSettings.Logger
	s.cancel()
	if s.connection != nil {
		err := s.connection.Close()
		if err != nil {
			logger.Error("connection close error", zap.Error(err))
		}
	}
	return nil
}
