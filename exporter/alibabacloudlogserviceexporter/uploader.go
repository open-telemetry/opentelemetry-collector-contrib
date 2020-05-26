// Copyright 2020, OpenTelemetry Authors
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

package alibabacloudlogserviceexporter

import (
	"errors"
	"net"
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/aliyun/aliyun-log-go-sdk/producer"
	"go.uber.org/zap"
)

type callback struct {
	logger *zap.Logger
}

func (callback *callback) Success(result *producer.Result) {
}

func (callback *callback) Fail(result *producer.Result) {
	callback.logger.Warn("send to log service failed", zap.String("requestId", result.GetRequestId()), zap.String("errorCode", result.GetErrorCode()), zap.String("errorMessage", result.GetErrorMessage()))
}

// Producer log Service's producer wrapper
type Producer interface {
	SendLogs(logs []*sls.Log) error
}

type producerImpl struct {
	producerInstance *producer.Producer
	project          string
	logstore         string
	topic            string
	source           string
	logger           *zap.Logger
}

func getIPAddress() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("connected to the network?")
}

// NewProducer Create Log Service Producer
func NewProducer(config *Config, logger *zap.Logger) (Producer, error) {
	if config == nil || config.Endpoint == "" || config.Project == "" || config.Logstore == "" {
		return nil, errors.New("missing logservice params: Endpoint, Project, Logstore")
	}
	// Retrieve Log Service Configuration Details
	producerConfig := producer.GetDefaultProducerConfig()
	producerConfig.Endpoint = config.Endpoint
	producerConfig.AccessKeyID = config.AccessKeyID
	producerConfig.AccessKeySecret = config.AccessKeySecret

	if config.MaxRetry > 0 {
		producerConfig.Retries = config.MaxRetry
	}
	if config.MaxBufferSize > 0 {
		producerConfig.TotalSizeLnBytes = int64(config.MaxBufferSize)
	}

	//Create Producer Instance
	producerInstance := producer.InitProducer(producerConfig)
	producerInstance.Start()
	p := &producerImpl{
		producerInstance: producerInstance,
	}
	p.project = config.Project
	p.logstore = config.Logstore
	// do not return error if get hostname or ip address fail
	var err error
	if p.topic, err = os.Hostname(); err != nil {
		logger.Warn("Get hostname error when create LogService producer", zap.Error(err))
	}
	if p.source, err = getIPAddress(); err != nil {
		logger.Warn("Get IP address error when create LogService producer", zap.Error(err))
	}
	p.logger = logger
	logger.Info("Create LogService producer success", zap.String("project", config.Project), zap.String("logstore", config.Logstore))

	return p, nil
}

// SendMessage Send message to LogService
func (producerInstance *producerImpl) SendLogs(logs []*sls.Log) error {
	return producerInstance.producerInstance.SendLogListWithCallBack(
		producerInstance.project,
		producerInstance.logstore,
		producerInstance.topic,
		producerInstance.source,
		logs,
		&callback{
			logger: producerInstance.logger,
		})
}
