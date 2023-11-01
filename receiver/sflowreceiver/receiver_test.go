// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
package sflowreceiver

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

func Test_sflowreceiverlogs_Start(t *testing.T) {
	type fields struct {
		host           component.Host
		cancel         context.CancelFunc
		nextConsumer   consumer.Logs
		config         *Config
		createSettings receiver.CreateSettings
		connection     *net.UDPConn
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test",
			fields: fields{
				config: &Config{
					confignet.NetAddr{
						Endpoint: "0.0.0.0:9995",
					},
					map[string]string{},
				},
				nextConsumer: consumertest.NewNop(),
				host:         componenttest.NewNopHost(),
				createSettings: receiver.CreateSettings{
					ID: component.ID{},
					TelemetrySettings: component.TelemetrySettings{
						Logger: func() *zap.Logger {
							log, _ := zap.NewDevelopment()
							return log
						}(),
						TracerProvider: nil,
						MeterProvider:  nil,
						MetricsLevel:   0,
						Resource:       pcommon.Resource{},
					},
					BuildInfo: component.BuildInfo{},
				},
			},
			args: args{
				ctx: func() context.Context {
					ctx, _ := context.WithTimeout(context.Background(), 50*time.Millisecond)
					return ctx
				}(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &sflowreceiverlogs{
				host:           tt.fields.host,
				cancel:         tt.fields.cancel,
				nextConsumer:   tt.fields.nextConsumer,
				config:         tt.fields.config,
				createSettings: tt.fields.createSettings,
				connection:     tt.fields.connection,
			}

			go startSflowReceiver(tt.args.ctx, s, t)
			sendSflowPacket(tt.args.ctx, s)

			time.Sleep(1 * time.Second)
			<-tt.args.ctx.Done()

			ctx, _ := context.WithTimeout(context.Background(), 50*time.Millisecond)
			err := s.Shutdown(ctx)
			assert.Nil(t, err)
		})
	}
}

func startSflowReceiver(ctx context.Context, s *sflowreceiverlogs, t *testing.T) {
	err := s.Start(ctx, s.host)
	if err != nil {
		s.Shutdown(ctx)
	}
	assert.Nil(t, err)
}

func sendSflowPacket(ctx context.Context, s *sflowreceiverlogs) {
	serverAddr, err := net.ResolveUDPAddr("udp", "0.0.0.0:9995")
	if err != nil {
		fmt.Println("Error resolving UDP address:", err)
		os.Exit(1)
	}

	// Create a UDP connection
	udpConn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Println("Error creating UDP connection:", err)
		os.Exit(1)
	}
	defer udpConn.Close()

	hexStream := "00000005000000010a060196000000100000a9b0123cbd380000000300000001000000d0000ede9c0000027f000020000a6a0600000000000000027f000002050000000200000001000000900000000100000624000000040000008094f7ad843841a4515e4e87358100045208004500060e297100003f11cd10c0a8ff06c0a8ff0511e312b505fa000008000000002afe0062f4640684d20050568e6e6d0800450005dcf6bd40003f04d31cac1e0a04ac1e0a03450005c823e440003f06cad5c0a8f91dc0a8cd07996a5ea0461e6182d4d1cbf2801001fbf1190000000003e9000000100000045200000000000004520000000000000001000000d0000ede9d0000027f000020000a6a2600000000000000027f000002050000000200000001000000900000000100000624000000040000008094f7ad843841a4515e4e87358100045208004500060e29c300003f11ccbec0a8ff06c0a8ff05ddfe12b505fa000008000000002afe0062f4640684d20050568e6e6d0800450005dcf70f40003f04d2caac1e0a04ac1e0a03450005c8d59a40003f06191fc0a8f91dc0a8cd07996c5ea0b1523a9d28dadfe7801001fb3c690000000003e9000000100000045200000000000004520000000000000001000000d0000ede9e0000027f000020000a6a4600000000000000027f000002050000000200000001000000900000000100000624000000040000008094f7ad843841a4515e4e87358100045208004500060e29c500003f11ccbcc0a8ff06c0a8ff05ddfe12b505fa000008000000002afe0062f4640684d20050568e6e6d0800450005dcf71140003f04d2c8ac1e0a04ac1e0a03450005c8d59c40003f06191dc0a8f91dc0a8cd07996c5ea0b15245c528dadfe7801001fb7f4e0000000003e90000001000000452000000000000045200000000"

	message, err := hex.DecodeString(hexStream)
	if err != nil {
		fmt.Println("error during hex decode", err)
	}

	_, err = udpConn.Write(message)
	if err != nil {
		fmt.Println("Error sending UDP message:", err)
		os.Exit(1)
	}
}
