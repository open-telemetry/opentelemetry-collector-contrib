// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	clssdk "github.com/tencentcloud/tencentcloud-cls-sdk-go"
	"go.uber.org/zap"
)

// logServiceClient log Service's client wrapper
type logServiceClient interface {
	// sendLogs send message to LogService
	sendLogs(logs []*clssdk.Log) error
}

type logServiceClientImpl struct {
	clientInstance *clssdk.AsyncProducerClient
	logset         string
	topic          string
	logger         *zap.Logger
}

// newLogServiceClient Create Log Service client
func newLogServiceClient(config *Config, logger *zap.Logger) (logServiceClient, error) {
	producerConfig := clssdk.GetDefaultAsyncProducerClientConfig()
	if config.Endpoint != "" {
		producerConfig.Endpoint = config.Endpoint
	} else {
		networkType := clssdk.Extranet
		if config.IsIntranet {
			networkType = clssdk.Intranet
		}
		producerConfig.SetEndpointByRegionAndNetworkType(clssdk.Region(config.Region), networkType)
	}

	producerConfig.AccessKeyID = config.SecretID
	producerConfig.AccessKeySecret = string(config.SecretKey)
	producerConfig.AccessToken = ""
	producerConfig.Retries = 10
	producerInstance, err := clssdk.NewAsyncProducerClient(producerConfig)
	if err != nil {
		return nil, err
	}
	producerInstance.Start()

	c := &logServiceClientImpl{
		clientInstance: producerInstance,
		logset:         config.LogSet,
		topic:          config.Topic,
		logger:         logger,
	}
	logger.Info("Create CLS LogService client success", zap.String("logset", config.LogSet), zap.String("topic", config.Topic))
	return c, nil
}

// sendLogs send message to LogService
func (c *logServiceClientImpl) sendLogs(logs []*clssdk.Log) error {
	return c.clientInstance.SendLogList(c.topic, logs, c)
}

// Success log service send log success
func (c *logServiceClientImpl) Success(result *clssdk.Result) {
	c.logger.Debug("Send to CLS LogService success",
		zap.String("topic", c.topic),
		zap.String("logset", c.logset),
		zap.String("request_id", result.GetRequestId()))
}

// Fail log service send log failed
func (c *logServiceClientImpl) Fail(result *clssdk.Result) {
	c.logger.Warn("Send to CLS LogService failed",
		zap.String("topic", c.topic),
		zap.String("logset", c.logset),
		zap.String("code", result.GetErrorCode()),
		zap.String("error_message", result.GetErrorMessage()),
		zap.String("request_id", result.GetRequestId()))
}
