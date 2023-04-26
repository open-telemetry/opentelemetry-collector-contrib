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

package tests

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestSplunkEventMetadataAsString(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
		What is this test doing:
		- Check text event received via event endpoint
	*/
	expectedEvents := 1
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: check text event received via event endpoint --")
	testData := "test data - text event for Splunk"
	index := viperConfigVariable("config.EVENT_INDEX_MAIN")
	source := "source_test_1"
	sourcetype := "sourcetype_test_1"
	sendTextEvent(index, source, sourcetype, testData)

	searchQuery := fmt.Sprintf("index=%s sourcetype=%s source=%s", index, sourcetype, source)
	query := searchQuery
	events := checkEventsFromSplunk(query, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(events))

	assert.True(t, len(events) >= expectedEvents, "Expected at least %d events, but received %d events", expectedEvents, len(events))
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestSplunkEventJsonObject(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
	   What is this test doing
	   - check json event received via event endpoint
	*/
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: check text event received via event endpoint --")

	expectedEvents := 1
	index := viperConfigVariable("config.EVENT_INDEX_NON_DEFAULT")
	source := "source_test_json"
	sourcetype := "sourcetype_test_json"
	data := map[string]interface{}{
		"test_name": "JSON Object",
		"field1":    "test",
		"field2":    155,
		"index":     index,
	}

	sendJsonEvent(index, source, sourcetype, data)
	searchQuery := fmt.Sprintf("index=%s sourcetype=%s source=%s", index, sourcetype, source)
	query := searchQuery
	events := checkEventsFromSplunk(query, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(events))

	assert.True(t, len(events) >= expectedEvents, "Expected at least %d events, but received %d events", expectedEvents, len(events))
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestSplunkEventTimestamp(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
	   What is this test doing
	   - check event timestamp test
	*/

	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: check text event received via event endpoint --")
	expectedEvents := 1
	index := viperConfigVariable("config.EVENT_TIMESTAMP")
	source := "source_test_3"
	sourcetype := "sourcetype_test_3"
	now := time.Now()
	start_time := now.Unix()
	time_shifted := now.Unix() - 1800
	logger.Println("UNIX: " + strconv.FormatInt(int64(start_time), 10) + " Shifted: " + strconv.FormatInt(int64(time_shifted), 10))
	data := map[string]interface{}{
		"time":      time_shifted,
		"test_name": "Test timestamp",
		"index":     index,
	}

	sendEventWithTimestamp(index, source, sourcetype, data, time_shifted)
	searchQuery := fmt.Sprintf("index=%s sourcetype=%s source=%s", index, sourcetype, source)
	query := searchQuery
	latestEvents := checkEventsFromSplunk(query, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(latestEvents))

	assert.True(t, len(latestEvents) == 0, "Expected no events, but received %d events", len(latestEvents))

	startTimePast := "-31m@m"
	endTimePast := "-29m@m"
	eventsInThePast := checkEventsFromSplunk(query, startTimePast, endTimePast)
	logger.Printf("Splunk received %d events in the last minute", len(eventsInThePast))

	assert.True(t, len(eventsInThePast) == expectedEvents)

	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestSplunkMetrics(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
	   What is this test doing
	   - check metrics via metrics endpoint test
	*/
	// expectedEvents := 1
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: check metrics via metrics endpoint test --")

	index := viperConfigVariable("config.METRICS_INDEX_METRICS_ENDPOINT")
	source := "source_test_metric"
	sourcetype := "sourcetype_test_metric"
	sendMetricEvent(index, source, sourcetype)

	metricName := viperConfigVariable("config.METRIC_NAME")
	events := checkMetricsFromSplunk(index, metricName)

	assert.True(t, len(events) >= 1, "Events length is less than 1. No metrics found")
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestSplunkHostMetricsEvents(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
	   What is this test doing
	    - host metrics receiver test
	*/
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: check metrics via metrics endpoint test --")

	index := viperConfigVariable("config.METRICS_INDEX_HOST_METRICS")
	metricsList := [...]string{
		"system.filesystem.inodes.usage",
		"system.filesystem.usage",
		"system.memory.usage",
		"system.network.connections",
		"system.network.dropped",
		"system.network.errors",
		"system.network.io",
		"system.network.packets",
		"system.cpu.load_average.1m",
		"system.cpu.load_average.5m",
		"system.cpu.load_average.15m",
	}

	for _, element := range metricsList {
		logger.Println("--- Checking metric events: " + element)
		events := checkMetricsFromSplunk(index, element)
		assert.True(t, len(events) >= 1, "Events length is less than 1. No metrics found")
	}
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestSplunkFileLogEventReceived(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
		What is this test doing:
		- Check events received via file log receiver
	*/
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: file log receiver test! --")
	expectedEvents := 1
	index := viperConfigVariable("config.EVENT_INDEX_FILE_LOG")
	sourcetype := "filelog_sourcetype"
	data := "File log test data - single line event"
	addTextEvent(data)
	searchQuery := fmt.Sprintf("index=%s sourcetype=%s", index, sourcetype)
	logger.Println("Search query " + searchQuery)
	events := checkEventsFromSplunk(searchQuery, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(events))

	assert.True(t, len(events) >= expectedEvents, "Expected at least %d events, but received %d events", expectedEvents, len(events))
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestSplunkFileLogEventReceivedJsonEvent(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
		What is this test doing:
		- Check events received via file log receiver
	*/
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: file log receiver test! --")
	expectedEvents := 1
	index := viperConfigVariable("config.EVENT_INDEX_FILE_LOG")
	sourcetype := "filelog_sourcetype"
	now := time.Now()
	eventUniqeTime := now.Unix()
	eventText := "json_event text " + strconv.FormatInt(int64(eventUniqeTime), 10)
	data := map[string]interface{}{"test_name": eventText, "source": "file_log"}
	addJsonTextEvent(data)
	searchQuery := fmt.Sprintf("index=%s sourcetype=%s %s", index, sourcetype, eventText)
	logger.Println("Search query " + searchQuery)
	events := checkEventsFromSplunk(searchQuery, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(events))

	assert.True(t, len(events) >= expectedEvents, "Expected at least %d events, but received %d events", expectedEvents, len(events))

	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

// Negative Tests
func TestNegativeSplunkDoesNotReceiveEvent_IvalidHecReceiverUrl(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
		What is this test doing:
		- check event not received when sent to Invalid Hec Receiver URL
	*/
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: Invalid Hec Receiver URL! --")
	expectedEvents := 0
	testData := "test data - text event for Splunk"
	index := viperConfigVariable("config.EVENT_INDEX_MAIN")
	source := "source_test_negative"
	sourcetype := "sourcetype_test_negative"
	INVALID_HEC_EVENTS_RECEIVER_URL := viperConfigVariable("config.INVALID_HEC_EVENTS_RECEIVER_URL")
	response_body := sendTextEventToUrl(index, source, sourcetype, testData, INVALID_HEC_EVENTS_RECEIVER_URL)

	assert.Equal(t, "\"OK\"", string(response_body))

	searchQuery := fmt.Sprintf("index=%s sourcetype=%s source=%s", index, sourcetype, source)
	query := searchQuery
	events := checkEventsFromSplunk(query, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(events))

	assert.True(t, len(events) == expectedEvents)
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}

func TestNegativeSplunkDoesNotReceiveEvent_DisabledHec(t *testing.T) {
	// t.Skip("Skipping test case execution")
	/*
		What is this test doing:
		- check event not received when sent to Disabled Hec
	*/
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Starting: Disabled Hec! --")
	expectedEvents := 0
	testData := "test data - text event for Splunk"
	index := viperConfigVariable("config.EVENT_INDEX_MAIN")
	source := "source_test_negative"
	sourcetype := "sourcetype_test_negative"
	DISABLED_HEC_EVENTS_RECEIVER_URL := viperConfigVariable("config.DISABLED_HEC_EVENTS_RECEIVER_URL")
	response_body := sendTextEventToUrl(index, source, sourcetype, testData, DISABLED_HEC_EVENTS_RECEIVER_URL)

	assert.Equal(t, "\"OK\"", string(response_body))

	searchQuery := fmt.Sprintf("index=%s sourcetype=%s source=%s", index, sourcetype, source)
	query := searchQuery
	events := checkEventsFromSplunk(query, viperConfigVariable("config.DEFAULT_SEARCH_START_TIME"))
	logger.Printf("Splunk received %d events in the last minute", len(events))

	assert.True(t, len(events) == expectedEvents)
	logger.Println("===")
	logger.Println("Test Finished")
	logger.Println("==============================================================================\n===")
}
