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

package mock

import (
	"context"
	"net"
	"testing"

	pb "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/proto/experimental/metrics/configservice"
	res "github.com/open-telemetry/opentelemetry-collector-contrib/extension/dynamicconfig/proto/resource/v1"
	"google.golang.org/grpc"
)

var GlobalFingerprint = []byte("There once was a cat named Gretchen")
var GlobalResponse = &pb.MetricConfigResponse{
	Fingerprint: GlobalFingerprint,
}

func AlterFingerprint(newFingerprint []byte) {
	GlobalFingerprint = newFingerprint
	GlobalResponse.Fingerprint = GlobalFingerprint
}

type Backend struct{}

func (*Backend) GetFingerprint(*res.Resource) ([]byte, error) {
	return []byte(GlobalFingerprint), nil
}

func (*Backend) BuildConfigResponse(*res.Resource) (*pb.MetricConfigResponse, error) {
	return GlobalResponse, nil
}

func (*Backend) Close() error {
	return nil
}

type Service struct {
	pb.UnimplementedMetricConfigServer
}

func (*Service) GetMetricConfig(context.Context, *pb.MetricConfigRequest) (*pb.MetricConfigResponse, error) {
	return GlobalResponse, nil
}

// startServer is a test utility to start a quick-n-dirty gRPC server using the
// mock backend.
func StartServer(t *testing.T, quit <-chan struct{}, done chan<- struct{}) string {
	listen, err := net.Listen("tcp", ":0")
	address := listen.Addr()

	if err != nil || listen == nil {
		t.Fatalf("fail to listen: %v", err)
	}

	server := grpc.NewServer()
	configService := &Service{}
	pb.RegisterMetricConfigServer(server, configService)

	go func() {
		if err := server.Serve(listen); err != nil {
			t.Errorf("fail to serve: %v", err)
		}
	}()

	go func() {
		<-quit
		server.Stop()

		done <- struct{}{}
	}()

	return address.String()
}
