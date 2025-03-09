// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudaomexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/huaweicloudaomexporter"

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/huaweicloud/huaweicloud-lts-sdk-go/producer"
	"go.uber.org/zap"
)

// logServiceClient log Service's client wrapper
type logServiceClient interface {
	// sendLogs send message to LogService
	sendLogs(logs []*producer.Log) error
}

type serviceClientImpl struct {
	proxyInstance  producer.ClientInterface
	clientInstance *producer.Producer
	projectId      string
	groupId        string
	streamId       string
	topic          string
	source         string
	logger         *zap.Logger
}

func getIPAddress() (ipAddress string, err error) {
	as, err := net.InterfaceAddrs()
	for _, a := range as {
		if in, ok := a.(*net.IPNet); ok && !in.IP.IsLoopback() {
			if in.IP.To4() != nil {
				ipAddress = in.IP.String()
			}
		}
	}
	return ipAddress, err
}

// newLogServiceClient Create Log Service client
func newLogServiceClient(config *Config, logger *zap.Logger) (logServiceClient, error) {
	if config == nil || config.Endpoint == "" || config.RegionId == "" || config.ProjectId == "" || config.LogGroupId == "" || config.LogStreamId == "" {
		return nil, errors.New("missing params: Endpoint, RegionId, ProjectId, LogGroupId, LogStreamId")
	}

	producerConfig := producer.GetConfig()
	producerConfig.Endpoint = config.Endpoint
	producerConfig.RegionId = config.RegionId
	producerConfig.ProjectId = config.ProjectId
	producerConfig.AccessKeyID = config.AccessKeyID
	producerConfig.AccessKeySecret = string(config.AccessKeySecret)

	c := &serviceClientImpl{
		projectId: config.ProjectId,
		groupId:   config.LogGroupId,
		streamId:  config.LogStreamId,
		logger:    logger,
	}
	if config.ProxyAddress != "" {
		logger.Info("use proxy for exporter. proxy address",
			zap.Int("size", len(config.ProxyAddress)))

		proxyAddress, err := url.Parse(config.ProxyAddress)
		if err != nil {
			logger.Warn("parse proxy fail", zap.Error(err))
			return nil, err
		}

		c.proxyInstance = producer.CreateNormalInterface(producerConfig)
		if pi, ok := c.proxyInstance.(*producer.Client); ok {
			pi.HTTPClient = &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyAddress),
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true,
					},
				},
			}
		}
	} else {
		c.clientInstance = producer.InitProducer(producerConfig)
		c.clientInstance.Start()
	}

	// do not return error if get hostname or ip address fail
	c.topic, _ = os.Hostname()
	c.source, _ = getIPAddress()
	logger.Info("Create client success",
		zap.String("projectId", config.ProjectId),
		zap.String("groupId", config.LogGroupId),
		zap.String("streamId", config.LogStreamId),
	)
	return c, nil
}

// sendLogs send message to LogService
func (c *serviceClientImpl) sendLogs(logs []*producer.Log) error {
	// proxy mod
	if c.proxyInstance != nil {
		lg := &producer.LogGroup{
			Logs: logs,
		}
		err := c.proxyInstance.PutLogs(c.groupId, c.streamId, lg)
		if err != nil {
			c.Fail(&producer.Result{})
		} else {
			c.Success(&producer.Result{})
		}
		return err
	}
	// no proxy
	for _, log := range logs {
		err := c.clientInstance.SendLogWithCallBack(c.groupId, c.streamId, log, c)
		if err != nil {
			c.logger.Warn("send log fail", zap.Error(err))
			return err
		}
	}
	return nil
}

// Success is impl of producer.CallBack
func (c *serviceClientImpl) Success(result *producer.Result) {
	c.logger.Info("Send Success",
		zap.String("requestId", result.GetRequestId()))
}

// Fail is impl of producer.CallBack
func (c *serviceClientImpl) Fail(result *producer.Result) {
	c.logger.Warn("Send to LogService failed",
		zap.String("project", c.projectId),
		zap.String("store", c.groupId),
		zap.String("code", result.GetErrorCode()),
		zap.String("error_message", result.GetErrorMessage()),
		zap.String("request_id", result.GetRequestId()))
}
