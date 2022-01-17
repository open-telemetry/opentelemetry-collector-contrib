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

	"go.opentelemetry.io/collector/model/pdata"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver/internal/metadata"
)

func (c *couchdbScraper) recordCouchdbAverageRequestTimeDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	averageRequestTimeMetricKey := []string{"request_time", "value", "arithmetic_mean"}
	averageRequestTimeValue, err := getValueFromBody(averageRequestTimeMetricKey, stats)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	if parsedValue, ok := c.parseFloat(averageRequestTimeMetricKey, averageRequestTimeValue); ok {
		c.mb.RecordCouchdbAverageRequestTimeDataPoint(now, parsedValue)
	}
}

func (c *couchdbScraper) recordCouchdbHttpdBulkRequestsDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	httpdBulkRequestsMetricKey := []string{"httpd", "bulk_requests", "value"}
	httpdBulkRequestsMetricValue, err := getValueFromBody(httpdBulkRequestsMetricKey, stats)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	if parsedValue, ok := c.parseInt(httpdBulkRequestsMetricKey, httpdBulkRequestsMetricValue); ok {
		c.mb.RecordCouchdbHttpdBulkRequestsDataPoint(now, parsedValue)
	}
}

func (c *couchdbScraper) recordCouchdbHttpdRequestsDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	methods := []string{metadata.AttributeHTTPMethod.COPY, metadata.AttributeHTTPMethod.DELETE, metadata.AttributeHTTPMethod.GET, metadata.AttributeHTTPMethod.HEAD, metadata.AttributeHTTPMethod.OPTIONS, metadata.AttributeHTTPMethod.POST, metadata.AttributeHTTPMethod.PUT}
	for _, method := range methods {
		httpdRequestMethodKey := []string{"httpd_request_methods", method, "value"}
		httpdRequestMethodValue, err := getValueFromBody(httpdRequestMethodKey, stats)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		if parsedValue, ok := c.parseInt(httpdRequestMethodKey, httpdRequestMethodValue); ok {
			c.mb.RecordCouchdbHttpdRequestsDataPoint(now, parsedValue, method)
		}
	}
}

func (c *couchdbScraper) recordCouchdbHttpdResponsesDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	codes := []string{"200", "201", "202", "204", "206", "301", "302", "304", "400", "401", "403", "404", "405", "406", "409", "412", "413", "414", "415", "416", "417", "500", "501", "503"}
	for _, code := range codes {
		httpdResponsetCodeKey := []string{"httpd_status_codes", code, "value"}
		httpdResponsetCodeValue, err := getValueFromBody(httpdResponsetCodeKey, stats)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		if parsedValue, ok := c.parseInt(httpdResponsetCodeKey, httpdResponsetCodeValue); ok {
			c.mb.RecordCouchdbHttpdResponsesDataPoint(now, parsedValue, code)
		}
	}
}

func (c *couchdbScraper) recordCouchdbHttpdViewsDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	views := []string{metadata.AttributeView.TemporaryViewReads, metadata.AttributeView.ViewReads}
	for _, view := range views {
		viewKey := []string{"httpd", view, "value"}
		viewValue, err := getValueFromBody(viewKey, stats)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		if parsedValue, ok := c.parseInt(viewKey, viewValue); ok {
			c.mb.RecordCouchdbHttpdViewsDataPoint(now, parsedValue, view)
		}
	}
}

func (c *couchdbScraper) recordCouchdbDatabaseOpenDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	openDatabaseKey := []string{"open_databases", "value"}
	openDatabaseMetricValue, err := getValueFromBody(openDatabaseKey, stats)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	if parsedValue, ok := c.parseInt(openDatabaseKey, openDatabaseMetricValue); ok {
		c.mb.RecordCouchdbDatabaseOpenDataPoint(now, parsedValue)
	}
}

func (c *couchdbScraper) recordCouchdbFileDescriptorOpenDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	fileDescriptorKey := []string{"open_os_files", "value"}
	fileDescriptorMetricValue, err := getValueFromBody(fileDescriptorKey, stats)
	if err != nil {
		errors.AddPartial(1, err)
		return
	}

	if parsedValue, ok := c.parseInt(fileDescriptorKey, fileDescriptorMetricValue); ok {
		c.mb.RecordCouchdbFileDescriptorOpenDataPoint(now, parsedValue)
	}
}

func (c *couchdbScraper) recordCouchdbDatabaseOperationsDataPoint(now pdata.Timestamp, stats map[string]interface{}, errors scrapererror.ScrapeErrors) {
	operations := []string{metadata.AttributeOperation.Reads, metadata.AttributeOperation.Writes}
	keyPaths := [][]string{{"database_reads", "value"}, {"database_writes", "value"}}
	for i := 0; i < len(operations); i++ {
		key := keyPaths[i]
		value, err := getValueFromBody(key, stats)
		if err != nil {
			errors.AddPartial(1, err)
			continue
		}

		if parsedValue, ok := c.parseInt(key, value); ok {
			c.mb.RecordCouchdbDatabaseOperationsDataPoint(now, parsedValue, operations[i])
		}
	}
}

func getValueFromBody(keys []string, body map[string]interface{}) (interface{}, error) {
	var currentValue interface{} = body
	for _, key := range keys {
		currentBody, ok := currentValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("could not find key in body")
		}

		currentValue, ok = currentBody[key]
		if !ok {
			return nil, fmt.Errorf("could not find key in body")
		}
	}
	return currentValue, nil
}

func (c *couchdbScraper) parseInt(key []string, value interface{}) (int64, bool) {
	switch i := value.(type) {
	case int64:
		c.logInvalid("int", key, value)
		return i, true
	case float64:
		c.logInvalid("int", key, value)
		return int64(i), true
	}
	return 0, false
}

func (c *couchdbScraper) parseFloat(key []string, value interface{}) (float64, bool) {
	switch f := value.(type) {
	case float64:
		c.logInvalid("float", key, value)
		return f, true
	}
	return 0, false
}

func (c *couchdbScraper) logInvalid(expectedType string, key []string, value interface{}) {
	c.logger.Info(
		"invalid value",
		zap.String("expectedType", expectedType),
		zap.Strings("key", key),
		zap.Any("value", value),
	)
}
