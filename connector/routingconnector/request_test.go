// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package routingconnector // import "github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector"

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/collector/client"
	"google.golang.org/grpc/metadata"
	"gopkg.in/yaml.v3"
)

type ctxConfig struct {
	GRPC map[string]string   `mapstructure:"grpc"`
	HTTP map[string][]string `mapstructure:"http"`
}

func createContextFromFile(t *testing.T, filePath string) (context.Context, error) {
	t.Helper()

	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var cfg ctxConfig
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}

	ctx := context.Background()
	if len(cfg.GRPC) > 0 {
		ctx = metadata.NewIncomingContext(ctx, metadata.New(cfg.GRPC))
	}
	if len(cfg.HTTP) > 0 {
		ctx = client.NewContext(ctx, client.Info{Metadata: client.NewMetadata(cfg.HTTP)})
	}
	return ctx, nil
}
