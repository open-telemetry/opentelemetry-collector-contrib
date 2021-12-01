// Copyright 2021, OpenTelemetry Authors
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

package coralogixexporter

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"

	cxpb "github.com/coralogix/opentelemetry-cx-protobuf-api/coralogixpb"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

// CoralogixClient by Coralogix
type CoralogixClient struct {
	cfg      Config
	logger   *zap.Logger
	cfggrpc  configgrpc.GRPCClientSettings
	conn     *grpc.ClientConn
	client   cxpb.CollectorServiceClient
	metadata cxpb.Metadata
}

// NewCoralogixClient by Coralogix
func NewCoralogixClient(cfg *Config, logger *zap.Logger) *CoralogixClient {
	// check env variables
	endpoint := cfg.Endpoint
	privateKey := cfg.PrivateKey
	appName := cfg.AppName
	subSystem := cfg.SubSystem

	if os.Getenv("CORALOGIX_ENDPOINT") != "" {
		endpoint = os.Getenv("CORALOGIX_ENDPOINT")
	}
	if os.Getenv("CORALOGIX_PRIVATE_KEY") != "" {
		privateKey = os.Getenv("CORALOGIX_PRIVATE_KEY")
	}
	if os.Getenv("CORALOGIX_APPLICATION_NAME") != "" {
		appName = os.Getenv("CORALOGIX_APPLICATION_NAME")
	}
	if os.Getenv("CORALOGIX_SUBSYSTEM_NAME") != "" {
		subSystem = os.Getenv("CORALOGIX_SUBSYSTEM_NAME")
	}

	c := &CoralogixClient{
		cfg:    *cfg,
		logger: logger,
		cfggrpc: configgrpc.GRPCClientSettings{
			Endpoint:    endpoint,
			Compression: "",
			TLSSetting: configtls.TLSClientSetting{
				TLSSetting:         configtls.TLSSetting{},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			ReadBufferSize:  0,
			WriteBufferSize: 0,
			WaitForReady:    false,
			Headers:         map[string]string{"ACCESS_TOKEN": privateKey, "appName": appName, "subsystemName": subSystem},
			BalancerName:    "",
		},
		metadata: cxpb.Metadata{
			ApplicationName: appName,
			SubsystemName:   subSystem,
		},
	}
	c.startConnection()
	return c
}
func (c *CoralogixClient) startConnection() {
	confTLS := &tls.Config{
		InsecureSkipVerify: false,
	}
	conn, err := grpc.Dial(c.cfggrpc.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(confTLS)))
	if err != nil {
		c.logger.Debug("connection error cannot dial to the endpoint ", zap.Error(err))
		fmt.Print(err)
	}
	c.conn = conn
	c.client = cxpb.NewCollectorServiceClient(c.conn)
	fmt.Println("CONN STATE IS :" + c.conn.GetState().String())
	c.logger.Debug("CONN STATE IS :" + c.conn.GetState().String())
}

func (c *CoralogixClient) newPost(td pdata.Traces) {
	// new: jaeger format and grpc sends
	// fmt.Println("================== TEST JAGER AND GRPC =====================")

	batches, err := jaeger.InternalTracesToJaegerProto(td)
	if err != nil {
		c.logger.Error("cant translate to jaeger proto", zap.Error(err))
		fmt.Printf("cant translate to jaeger proto\n")
		fmt.Println(err)
	}
	allStr := ""

	ctx := metadata.NewOutgoingContext(context.TODO(), metadata.New(c.cfggrpc.Headers))

	for _, batch := range batches {
		fmt.Println(batch.String())
		allStr += batch.String()
		_, err := c.client.PostSpans(ctx, &cxpb.PostSpansRequest{Batch: *batch, Metadata: &c.metadata}, grpc.WaitForReady(false))
		if err != nil {
			c.logger.Error("failed to push trace data via Jaeger exporter", zap.Error(err))
			fmt.Printf("failed to push trace data via Jaeger exporter")
			fmt.Print(err)
		}
	}
	// fmt.Println("================== TEST JAGER AND GRPC =====================")
}
