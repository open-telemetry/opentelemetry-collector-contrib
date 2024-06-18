// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solarwindsapmsettingsextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension"

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/solarwinds/apm-proto/go/collectorpb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	jsonOutputFile      = "/tmp/solarwinds-apm-settings.json"
	grpcContextDeadline = 1 * time.Second
)

type solarwindsapmSettingsExtension struct {
	logger *zap.Logger
	config *Config
	cancel context.CancelFunc
	conn   *grpc.ClientConn
	client collectorpb.TraceCollectorClient
}

func newSolarwindsApmSettingsExtension(extensionCfg *Config, logger *zap.Logger) (extension.Extension, error) {
	settingsExtension := &solarwindsapmSettingsExtension{
		config: extensionCfg,
		logger: logger,
	}
	return settingsExtension, nil
}

func (extension *solarwindsapmSettingsExtension) Start(_ context.Context, _ component.Host) error {
	extension.logger.Info("starting up solarwinds apm settings extension")
	ctx := context.Background()
	ctx, extension.cancel = context.WithCancel(ctx)
	var err error
	extension.conn, err = grpc.NewClient(extension.config.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
	if err != nil {
		return err
	}
	extension.logger.Info("dailed to endpoint", zap.String("endpoint", extension.config.Endpoint))
	extension.client = collectorpb.NewTraceCollectorClient(extension.conn)

	// initial refresh
	refresh(extension, jsonOutputFile)

	go func() {
		ticker := time.NewTicker(extension.config.Interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				refresh(extension, jsonOutputFile)
			case <-ctx.Done():
				extension.logger.Info("received ctx.Done() from ticker")
				return
			}
		}
	}()

	return nil
}

func (extension *solarwindsapmSettingsExtension) Shutdown(_ context.Context) error {
	extension.logger.Info("shutting down solarwinds apm settings extension")
	if extension.cancel != nil {
		extension.cancel()
	}
	if extension.conn != nil {
		return extension.conn.Close()
	}
	return nil
}

func refresh(extension *solarwindsapmSettingsExtension, filename string) {
	extension.logger.Info("time to refresh", zap.String("endpoint", extension.config.Endpoint))
	if hostname, err := os.Hostname(); err != nil {
		extension.logger.Error("unable to call os.Hostname()", zap.Error(err))
	} else {
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
			extension.logger.Error("unable to get settings", zap.String("endpoint", extension.config.Endpoint), zap.Error(err))
			return
		}
		switch result := response.GetResult(); result {
		case collectorpb.ResultCode_OK:
			if len(response.GetWarning()) > 0 {
				extension.logger.Warn("GetSettings succeed", zap.String("result", result.String()), zap.String("warning", response.GetWarning()))
			}
			var settings []map[string]any
			for _, item := range response.GetSettings() {
				marshalOptions := protojson.MarshalOptions{
					UseEnumNumbers:  true,
					EmitUnpopulated: true,
				}
				if settingBytes, err := marshalOptions.Marshal(item); err != nil {
					extension.logger.Warn("error to marshal setting JSON[] byte from response.GetSettings()", zap.Error(err))
				} else {
					setting := make(map[string]any)
					if err := json.Unmarshal(settingBytes, &setting); err != nil {
						extension.logger.Warn("error to unmarshal setting JSON object from setting JSON[]byte", zap.Error(err))
					} else {
						if value, ok := setting["value"].(string); ok {
							if num, err := strconv.ParseInt(value, 10, 0); err != nil {
								extension.logger.Warn("unable to parse value "+value+" as number", zap.Error(err))
							} else {
								setting["value"] = num
							}
						}
						if timestamp, ok := setting["timestamp"].(string); ok {
							if num, err := strconv.ParseInt(timestamp, 10, 0); err != nil {
								extension.logger.Warn("unable to parse timestamp "+timestamp+" as number", zap.Error(err))
							} else {
								setting["timestamp"] = num
							}
						}
						if ttl, ok := setting["ttl"].(string); ok {
							if num, err := strconv.ParseInt(ttl, 10, 0); err != nil {
								extension.logger.Warn("unable to parse ttl "+ttl+" as number", zap.Error(err))
							} else {
								setting["ttl"] = num
							}
						}
						if _, ok := setting["flags"]; ok {
							setting["flags"] = string(item.Flags)
						}
						if arguments, ok := setting["arguments"].(map[string]any); ok {
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
							// Remove SignatureKey from collector response
							delete(arguments, "SignatureKey")
						}
						settings = append(settings, setting)
					}
				}
			}
			if content, err := json.Marshal(settings); err != nil {
				extension.logger.Warn("error to marshal setting JSON[] byte from settings", zap.Error(err))
			} else {
				if err := os.WriteFile(filename, content, 0600); err != nil {
					extension.logger.Error("unable to write "+filename, zap.Error(err))
				} else {
					if len(response.GetWarning()) > 0 {
						extension.logger.Warn(filename + " is refreshed (soft disabled)")
					} else {
						extension.logger.Info(filename + " is refreshed")
					}
					extension.logger.Info(string(content))
				}
			}
		default:
			extension.logger.Warn("GetSettings failed", zap.String("result", result.String()), zap.String("warning", response.GetWarning()))
		}
	}
}
