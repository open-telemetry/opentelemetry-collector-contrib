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

package dynamicconfig

import (
	"context"
	"net"

	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/service"
	pb "github.com/open-telemetry/opentelemetry-proto/gen/go/experimental/metricconfigservice"
)

type dynamicConfigExtension struct {
	config        Config // extension-level configs
	logger        *zap.Logger
	server        *grpc.Server           // gRPC server that physically serves the ConfigService
	configService *service.ConfigService // implements the gRPC configuration service
}

func newServer(config Config, logger *zap.Logger) (*dynamicConfigExtension, error) {
	de := &dynamicConfigExtension{
		config: config,
		logger: logger,
		server: grpc.NewServer(),
	}

	return de, nil
}

func (de *dynamicConfigExtension) Start(ctx context.Context, host component.Host) error {
	de.logger.Info("Starting dynamic config extension", zap.Any("config", de.config))
	listen, err := net.Listen("tcp", de.config.Endpoint)
	if err != nil {
		return err
	}

	var configService *service.ConfigService
	if de.config.RemoteConfigAddress != "" {
		configService, err = service.NewConfigService(
			service.WithRemoteConfig(de.config.RemoteConfigAddress),
		)
	} else {
		configService, err = service.NewConfigService(
			service.WithFileConfig(de.config.LocalConfigFile),
			service.WithWaitTime(int32(de.config.WaitTime)),
		)
	}
	if err != nil {
		return err
	}

	de.configService = configService
	pb.RegisterMetricConfigServer(de.server, configService)

	go func() {
		if err := de.server.Serve(listen); err != nil {
			host.ReportFatalError(err)
		}
	}()

	return nil
}

func (de *dynamicConfigExtension) Shutdown(ctx context.Context) error {
	de.logger.Info("Shutting down dynamic config extension")
	de.configService.Stop()
	de.server.GracefulStop()
	return nil
}
