// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tencentcloudlogserviceexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter"

import (
	"github.com/pierrec/lz4"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tchttp "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/http"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	"go.uber.org/zap"
	pb "google.golang.org/protobuf/proto"

	cls "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter/proto"
)

// logServiceClient log Service's client wrapper
type logServiceClient interface {
	// sendLogs send message to LogService
	sendLogs(logs []*cls.Log) error
}

type logServiceClientImpl struct {
	clientInstance *common.Client
	logset         string
	topic          string
	hashkey        string
	logger         *zap.Logger
}

// newLogServiceClient Create Log Service client
func newLogServiceClient(config *Config, logger *zap.Logger) logServiceClient {
	credential := common.NewCredential(config.SecretID, string(config.SecretKey))

	c := &logServiceClientImpl{
		clientInstance: common.NewCommonClient(credential, config.Region, profile.NewClientProfile()),
		logset:         config.LogSet,
		topic:          config.Topic,
		logger:         logger,
	}

	logger.Info("Create LogService client success", zap.String("logset", config.LogSet), zap.String("topic", config.Topic))
	return c
}

// sendLogs send message to LogService
func (c *logServiceClientImpl) sendLogs(logs []*cls.Log) error {
	headers := map[string]string{
		"X-CLS-TopicId": c.topic,
		"X-CLS-HashKey": c.hashkey,
	}
	commpresstype := ""

	logGroup := cls.LogGroup{
		Logs: logs,
	}
	logGroupList := cls.LogGroupList{
		LogGroupList: []*cls.LogGroup{
			&logGroup,
		},
	}
	data, _ := pb.Marshal(&logGroupList)

	length := lz4.CompressBlockBound(len(data)) + 1
	compressbody := make([]byte, length)
	n, err := lz4.CompressBlock(data, compressbody, nil)
	if err == nil && n > 0 {
		commpresstype = "lz4"
		data = compressbody[0:n]
	}
	headers["X-CLS-CompressType"] = commpresstype

	request := tchttp.NewCommonRequest("cls", "2020-10-16", "UploadLog")
	request.SetOctetStreamParameters(headers, data)

	response := tchttp.NewCommonResponse()

	return c.clientInstance.SendOctetStream(request, response)
}
