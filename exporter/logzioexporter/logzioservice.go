// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package logzioexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter"

import (
	"fmt"
	"hash/fnv"

	"github.com/jaegertracing/jaeger/model"
)

const serviceLogType = "jaegerService"

// logzioService type, for query purposes
type logzioService struct {
	OperationName string `json:"operationName"`
	ServiceName   string `json:"serviceName"`
	Type          string `json:"type"`
}

// newLogzioService creates a new logzio service from a span
func newLogzioService(span *model.Span) logzioService {
	service := logzioService{
		ServiceName:   span.Process.ServiceName,
		OperationName: span.OperationName,
		Type:          serviceLogType,
	}
	return service
}

// HashCode receives a logzio service and returns a hash representation of it's service name and operation name.
func (service *logzioService) HashCode() (string, error) {
	hash := fnv.New64a()
	_, err := hash.Write(append([]byte(service.ServiceName), []byte(service.OperationName)...))
	return fmt.Sprintf("%x", hash.Sum64()), err
}
