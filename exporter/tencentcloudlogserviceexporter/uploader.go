// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	"context"
	"fmt"

	cls "github.com/tencentcloud/tencentcloud-cls-sdk-go"
	"go.uber.org/zap"
)

// shutdownTimeoutMs is the maximum time given to the producer to flush the logs
// still buffered while shutting down.
const shutdownTimeoutMs int64 = 10000

// logServiceClient log Service's client wrapper
type logServiceClient interface {
	// sendLogs send message to LogService
	sendLogs(logs []*cls.Log) error
	// shutdown flushes the logs buffered by the SDK and stops its background workers
	shutdown(context.Context) error
}

type logServiceClientImpl struct {
	clientInstance *cls.AsyncProducerClient
	logset         string
	topic          string
	logger         *zap.Logger
}

// newProducerConfig builds the CLS producer configuration. The endpoint is taken from
// Endpoint when set, otherwise it is derived from Region and the requested network type.
func newProducerConfig(config *Config) *cls.AsyncProducerClientConfig {
	producerConfig := cls.GetDefaultAsyncProducerClientConfig()
	if config.Endpoint != "" {
		producerConfig.Endpoint = config.Endpoint
	} else {
		networkType := cls.Extranet
		if config.IsIntranet {
			networkType = cls.Intranet
		}
		producerConfig.SetEndpointByRegionAndNetworkType(cls.Region(config.Region), networkType)
	}

	producerConfig.AccessKeyID = config.SecretID
	producerConfig.AccessKeySecret = string(config.SecretKey)
	return producerConfig
}

// newLogServiceClient Create Log Service client
func newLogServiceClient(config *Config, logger *zap.Logger) (logServiceClient, error) {
	producerConfig := newProducerConfig(config)

	producerInstance, err := cls.NewAsyncProducerClient(producerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create CLS LogService client: %w", err)
	}
	producerInstance.Start()

	c := &logServiceClientImpl{
		clientInstance: producerInstance,
		logset:         config.LogSet,
		topic:          config.Topic,
		logger:         logger,
	}

	logger.Info("Create CLS LogService client success",
		zap.String("endpoint", producerConfig.Endpoint),
		zap.String("logset", config.LogSet),
		zap.String("topic", config.Topic))
	return c, nil
}

// sendLogs hands the logs over to the SDK, which takes care of batching, compression
// and retries. The outcome of each batch is reported through the CallBack methods.
func (c *logServiceClientImpl) sendLogs(logs []*cls.Log) error {
	if len(logs) == 0 {
		return nil
	}
	return c.clientInstance.SendLogList(c.topic, logs, c)
}

// shutdown flushes the logs buffered by the SDK and stops its background workers.
func (c *logServiceClientImpl) shutdown(ctx context.Context) error {
	ch := make(chan error)
	go func() {
		ch <- c.clientInstance.Close(shutdownTimeoutMs)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-ch:
		return err
	}
}

// Success log service send log success
func (c *logServiceClientImpl) Success(result *cls.Result) {
	c.logger.Debug("Send to CLS LogService success",
		zap.String("logset", c.logset),
		zap.String("topic", c.topic),
		zap.String("request_id", result.GetRequestId()))
}

// Fail log service send log failed
func (c *logServiceClientImpl) Fail(result *cls.Result) {
	c.logger.Warn("Send to CLS LogService failed",
		zap.String("logset", c.logset),
		zap.String("topic", c.topic),
		zap.String("code", result.GetErrorCode()),
		zap.String("error_message", result.GetErrorMessage()),
		zap.String("request_id", result.GetRequestId()))
}
