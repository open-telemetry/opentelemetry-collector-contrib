// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"path"
	"time"

	"github.com/solarwindscloud/apm-proto/go/collectorpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	jsonOutputFile      = "solarwinds-apm-settings.json"
	grpcContextDeadline = 1 * time.Second
)

type solarwindsapmSettingsExtension struct {
	config            *Config
	cancel            context.CancelFunc
	conn              *grpc.ClientConn
	client            collectorpb.TraceCollectorClient
	telemetrySettings component.TelemetrySettings
}

func newSolarwindsApmSettingsExtension(extensionCfg *Config, settings extension.Settings) (extension.Extension, error) {
	settingsExtension := &solarwindsapmSettingsExtension{
		config:            extensionCfg,
		telemetrySettings: settings.TelemetrySettings,
	}
	return settingsExtension, nil
}

func (extension *solarwindsapmSettingsExtension) Start(_ context.Context, host component.Host) error {
	extension.telemetrySettings.Logger.Info("starting up solarwinds apm settings extension")
	ctx := context.Background()
	ctx, extension.cancel = context.WithCancel(ctx)
	systemCertPool, err := x509.SystemCertPool()
	if err != nil {
		return err
	}
	extension.conn, err = extension.config.ClientConfig.ToClientConn(ctx, host, extension.telemetrySettings, configgrpc.WithGrpcDialOption(grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{RootCAs: systemCertPool}))))
	if err != nil {
		return err
	}
	extension.telemetrySettings.Logger.Info("created a gRPC client", zap.String("endpoint", extension.config.ClientConfig.Endpoint))
	extension.client = collectorpb.NewTraceCollectorClient(extension.conn)

	outputFile := path.Join(os.TempDir(), jsonOutputFile)
	// initial refresh
	refresh(extension, outputFile)

	go func() {
		ticker := time.NewTicker(extension.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh(extension, outputFile)
			case <-ctx.Done():
				extension.telemetrySettings.Logger.Info("received ctx.Done() from ticker")
				return
			}
		}
	}()

	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.telemetrySettings.Logger.Info("shutting down solarwinds apm settings extension")
	if extension.cancel != nil {
		extension.cancel()
	}
	if extension.conn != nil {
		return extension.conn.Close()
	}
	return nil
}

func refresh(extension *solarwindsapmSettingsExtension, filename string) {
	extension.telemetrySettings.Logger.Info("time to refresh", zap.String("endpoint", extension.config.ClientConfig.Endpoint))
	hostname, err := os.Hostname()
	if err != nil {
		extension.telemetrySettings.Logger.Error("unable to call os.Hostname()", zap.Error(err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), grpcContextDeadline)
	defer cancel()

	request := &collectorpb.SettingsRequest{
		ApiKey: extension.config.Key,
		Identity: &collectorpb.HostID{
			Hostname: hostname,
		},
		ClientVersion: "2",
	}
	response, err := extension.client.GetSettings(ctx, request)
	if err != nil {
		extension.telemetrySettings.Logger.Error("unable to get settings", zap.String("endpoint", extension.config.ClientConfig.Endpoint), zap.Error(err))
		return
	}
	switch result := response.GetResult(); result {
	case collectorpb.ResultCode_OK:
		if len(response.GetWarning()) > 0 {
			extension.telemetrySettings.Logger.Warn("GetSettings succeed", zap.String("result", result.String()), zap.String("warning", response.GetWarning()))
		}
		var settings []map[string]any
		for _, item := range response.GetSettings() {
			setting := make(map[string]any)
			setting["flags"] = string(item.GetFlags())
			setting["timestamp"] = item.GetTimestamp()
			setting["value"] = item.GetValue()
			arguments := make(map[string]any)
			if value, ok := item.Arguments["BucketCapacity"]; ok {
				arguments["BucketCapacity"] = math.Float64frombits(binary.LittleEndian.Uint64(value))
			}
			if value, ok := item.Arguments["BucketRate"]; ok {
				arguments["BucketRate"] = math.Float64frombits(binary.LittleEndian.Uint64(value))
			}
			if value, ok := item.Arguments["TriggerRelaxedBucketCapacity"]; ok {
				arguments["TriggerRelaxedBucketCapacity"] = math.Float64frombits(binary.LittleEndian.Uint64(value))
			}
			if value, ok := item.Arguments["TriggerRelaxedBucketRate"]; ok {
				arguments["TriggerRelaxedBucketRate"] = math.Float64frombits(binary.LittleEndian.Uint64(value))
			}
			if value, ok := item.Arguments["TriggerStrictBucketCapacity"]; ok {
				arguments["TriggerStrictBucketCapacity"] = math.Float64frombits(binary.LittleEndian.Uint64(value))
			}
			if value, ok := item.Arguments["TriggerStrictBucketRate"]; ok {
				arguments["TriggerStrictBucketRate"] = math.Float64frombits(binary.LittleEndian.Uint64(value))
			}
			if value, ok := item.Arguments["MetricsFlushInterval"]; ok {
				arguments["MetricsFlushInterval"] = int32(binary.LittleEndian.Uint32(value))
			}
			if value, ok := item.Arguments["MaxTransactions"]; ok {
				arguments["MaxTransactions"] = int32(binary.LittleEndian.Uint32(value))
			}
			if value, ok := item.Arguments["MaxCustomMetrics"]; ok {
				arguments["MaxCustomMetrics"] = int32(binary.LittleEndian.Uint32(value))
			}
			if value, ok := item.Arguments["EventsFlushInterval"]; ok {
				arguments["EventsFlushInterval"] = int32(binary.LittleEndian.Uint32(value))
			}
			if value, ok := item.Arguments["ProfilingInterval"]; ok {
				arguments["ProfilingInterval"] = int32(binary.LittleEndian.Uint32(value))
			}
			setting["arguments"] = arguments
			setting["ttl"] = item.GetTtl()
			settings = append(settings, setting)
		}
		if content, err := json.Marshal(settings); err != nil {
			extension.telemetrySettings.Logger.Error("unable to marshal setting JSON[] byte from settings", zap.Error(err))
		} else {
			if err := os.WriteFile(filename, content, 0o600); err != nil {
				extension.telemetrySettings.Logger.Error("unable to write "+filename, zap.Error(err))
			} else {
				if len(response.GetWarning()) > 0 {
					extension.telemetrySettings.Logger.Warn(filename + " is refreshed (soft disabled)")
				} else {
					extension.telemetrySettings.Logger.Info(filename + " is refreshed")
				}
				extension.telemetrySettings.Logger.Info(string(content))
			}
		}
	default:
		extension.telemetrySettings.Logger.Warn("GetSettings failed", zap.String("result", result.String()), zap.String("warning", response.GetWarning()))
	}
}
