// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package couchdbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver"

import (
	"fmt"
	"strings"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

func (c *couchdbScraper) recordCouchdbAverageRequestTimeDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	averageRequestTimeMetricKey := []string{"request_time", "value", "arithmetic_mean"}
	averageRequestTimeValue, err := getFloatFromBody(averageRequestTimeMetricKey, stats)
	if err != nil {
		c.logger.Info(
			err.Error(),
			zap.String("key", strings.Join(averageRequestTimeMetricKey, " ")),
		)
		return
	}

	c.mb.RecordCouchdbAverageRequestTimeDataPoint(now, averageRequestTimeValue)
}

func (c *couchdbScraper) recordCouchdbHttpdBulkRequestsDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	httpdBulkRequestsMetricKey := []string{"httpd", "bulk_requests", "value"}
	httpdBulkRequestsMetricValue, err := getIntFromBody(httpdBulkRequestsMetricKey, stats)
	if err != nil {
		c.logger.Info(
			err.Error(),
			zap.String("key", strings.Join(httpdBulkRequestsMetricKey, " ")),
		)
		return
	}

	c.mb.RecordCouchdbHttpdBulkRequestsDataPoint(now, httpdBulkRequestsMetricValue)
}

func (c *couchdbScraper) recordCouchdbHttpdRequestsDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	methods := []string{metadata.AttributeHTTPMethod.COPY, metadata.AttributeHTTPMethod.DELETE, metadata.AttributeHTTPMethod.GET, metadata.AttributeHTTPMethod.HEAD, metadata.AttributeHTTPMethod.OPTIONS, metadata.AttributeHTTPMethod.POST, metadata.AttributeHTTPMethod.PUT}
	for _, method := range methods {
		httpdRequestMethodKey := []string{"httpd_request_methods", method, "value"}
		httpdRequestMethodValue, err := getIntFromBody(httpdRequestMethodKey, stats)
		if err != nil {
			c.logger.Info(
				err.Error(),
				zap.String("key", strings.Join(httpdRequestMethodKey, " ")),
			)
			return
		}

		c.mb.RecordCouchdbHttpdRequestsDataPoint(now, httpdRequestMethodValue, method)
	}
}

func (c *couchdbScraper) recordCouchdbHttpdResponsesDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	codes := []string{"200", "201", "202", "204", "206", "301", "302", "304", "400", "401", "403", "404", "405", "406", "409", "412", "413", "414", "415", "416", "417", "500", "501", "503"}
	for _, code := range codes {
		httpdResponsetCodeKey := []string{"httpd_status_codes", code, "value"}
		httpdResponsetCodeValue, err := getIntFromBody(httpdResponsetCodeKey, stats)
		if err != nil {
			c.logger.Info(
				err.Error(),
				zap.String("key", strings.Join(httpdResponsetCodeKey, " ")),
			)
			return
		}

		c.mb.RecordCouchdbHttpdResponsesDataPoint(now, httpdResponsetCodeValue, code)
	}
}

func (c *couchdbScraper) recordCouchdbHttpdViewsDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	views := []string{metadata.AttributeView.TemporaryViewReads, metadata.AttributeView.ViewReads}
	for _, view := range views {
		httpdResponsetCodeKey := []string{"httpd", view, "value"}
		httpdResponsetCodeValue, err := getIntFromBody(httpdResponsetCodeKey, stats)
		if err != nil {
			c.logger.Info(
				err.Error(),
				zap.String("key", strings.Join(httpdResponsetCodeKey, " ")),
			)
			return
		}

		c.mb.RecordCouchdbHttpdViewsDataPoint(now, httpdResponsetCodeValue, view)
	}
}

func (c *couchdbScraper) recordCouchdbDatabaseOpenDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	openDatabaseKey := []string{"open_databases", "value"}
	openDatabaseMetricValue, err := getIntFromBody(openDatabaseKey, stats)
	if err != nil {
		c.logger.Info(
			err.Error(),
			zap.String("key", strings.Join(openDatabaseKey, " ")),
		)
		return
	}

	c.mb.RecordCouchdbDatabaseOpenDataPoint(now, openDatabaseMetricValue)
}

func (c *couchdbScraper) recordCouchdbFileDescriptorOpenDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	fileDescriptorKey := []string{"open_os_files", "value"}
	fileDescriptorMetricValue, err := getIntFromBody(fileDescriptorKey, stats)
	if err != nil {
		c.logger.Info(
			err.Error(),
			zap.String("key", strings.Join(fileDescriptorKey, " ")),
		)
		return
	}

	c.mb.RecordCouchdbFileDescriptorOpenDataPoint(now, fileDescriptorMetricValue)
}

func (c *couchdbScraper) recordCouchdbDatabaseOperationsDataPoint(now pdata.Timestamp, stats map[string]interface{}) {
	databaseOperationsReadsKey := []string{"database_reads", "value"}
	databaseOperationsReadsValue, err := getIntFromBody(databaseOperationsReadsKey, stats)
	if err != nil {
		c.logger.Info(
			err.Error(),
			zap.String("key", strings.Join(databaseOperationsReadsKey, " ")),
		)
	}

	c.mb.RecordCouchdbDatabaseOperationsDataPoint(now, databaseOperationsReadsValue, metadata.AttributeOperation.Reads)

	databaseOperationsWritesKey := []string{"database_writes", "value"}
	databaseOperationsWritesValue, err := getIntFromBody(databaseOperationsWritesKey, stats)
	if err != nil {
		c.logger.Info(
			err.Error(),
			zap.String("key", strings.Join(databaseOperationsWritesKey, " ")),
		)
		return
	}

	c.mb.RecordCouchdbDatabaseOperationsDataPoint(now, databaseOperationsWritesValue, metadata.AttributeOperation.Writes)
}

func getFloatFromBody(keys []string, body map[string]interface{}) (float64, error) {
	var currentValue interface{} = body

	for _, key := range keys {
		currentBody, ok := currentValue.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("could not find key in body")
		}

		currentValue, ok = currentBody[key]
		if !ok {
			return 0, fmt.Errorf("could not find key in body")
		}
	}
	floatVal, ok := parseFloat(currentValue)
	if !ok {
		return 0, fmt.Errorf("could not parse value as float")
	}
	return floatVal, nil
}

func parseFloat(value interface{}) (float64, bool) {
	switch f := value.(type) {
	case float64:
		return f, true
	}
	return 0, false
}

func parseInt(value interface{}) (int64, bool) {
	switch i := value.(type) {
	case float64:
		return int64(i), true
	}
	return 0, false
}

func getIntFromBody(keys []string, body map[string]interface{}) (int64, error) {
	var currentValue interface{} = body

	for _, key := range keys {
		currentBody, ok := currentValue.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("could not find key in body")
		}

		currentValue, ok = currentBody[key]
		if !ok {
			return 0, fmt.Errorf("could not find key in body")
		}
	}
	intVal, ok := parseInt(currentValue)
	if !ok {
		return 0, fmt.Errorf("could not parse value as int")
	}
	return intVal, nil
}
