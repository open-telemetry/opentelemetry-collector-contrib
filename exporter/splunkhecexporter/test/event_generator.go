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

package tests // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter/tests"
import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func sendTextEvent(index string, source string, sourcetype string, testData string) {
	//Encode the data
	postBody, _ := json.Marshal(map[string]string{
		"event":      testData,
		"index":      index,
		"host":       "localhost",
		"source":     source,
		"sourcetype": sourcetype,
	})
	host := viperConfigVariable("config.EVENTS_RECEIVER_URL")
	responseBody := bytes.NewBuffer(postBody)
	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(host, "application/json", responseBody)
	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Send Event Response Body: " + string(body))
	waitForEventToBeIndexedInSplunk()
}

func sendTextEventToUrl(index string, source string, sourcetype string, testData string, eventsReceiverUrl string) []byte {
	//Encode the data
	postBody, _ := json.Marshal(map[string]string{
		"event":      testData,
		"index":      index,
		"host":       "localhost",
		"source":     source,
		"sourcetype": sourcetype,
	})
	responseBody := bytes.NewBuffer(postBody)
	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(eventsReceiverUrl, "application/json", responseBody)
	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	log.Println("Send text event Status Code: " + strconv.FormatInt(int64(resp.StatusCode), 10))
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Send Event Response Body: " + string(body))
	waitForEventToBeIndexedInSplunk()
	return body
}

func sendJsonEvent(index string, source string, sourcetype string, testData map[string]interface{}) {
	//Encode the data
	postBody, _ := json.Marshal(map[string]interface{}{
		"event":      testData,
		"index":      index,
		"host":       "localhost",
		"source":     source,
		"sourcetype": sourcetype,
	})
	responseBody := bytes.NewBuffer(postBody)
	host := viperConfigVariable("config.EVENTS_RECEIVER_URL")
	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(host, "application/json", responseBody)
	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Send Event Response Body: " + string(body))
	waitForEventToBeIndexedInSplunk()
}

func sendEventWithTimestamp(index string, source string, sourcetype string, testData map[string]interface{}, event_timestamp int64) {
	//Encode the data
	postBody, _ := json.Marshal(map[string]interface{}{
		"event":      testData,
		"time":       event_timestamp,
		"index":      index,
		"host":       "localhost",
		"source":     source,
		"sourcetype": sourcetype,
	})
	host := viperConfigVariable("config.EVENTS_RECEIVER_URL")
	responseBody := bytes.NewBuffer(postBody)
	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(host, "application/json", responseBody)
	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	log.Printf("Send Event Response Body: " + string(body))
	waitForEventToBeIndexedInSplunk()
}

func sendMetricEvent(index string, source string, sourcetype string) {
	//Encode the data
	log.Printf("Sending metric event")
	fields := map[string]interface{}{
		"metric_name:" + viperConfigVariable("config.METRIC_NAME"): 123,
		"k0":                  "v0",
		"k1":                  "v1",
		"metric_name:cpu.usr": 11.12,
		"metric_name:cpu.sys": 12.23,
	}
	jsonStr, err := json.Marshal(map[string]interface{}{
		"index":      index,
		"host":       "localhost",
		"source":     source,
		"sourcetype": sourcetype,
		"event":      "metric",
		"fields":     fields,
	})
	if err != nil {
		panic(err)
	}

	requestBody := bytes.NewBuffer(jsonStr)
	host := viperConfigVariable("config.METRICS_RECEIVER_URL")
	req, err := http.NewRequest("POST", host, requestBody)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	log.Printf("Send Metric Status Code: " + strconv.Itoa(resp.StatusCode) + "| Response Body: " + string(bodyBytes))
	waitForEventToBeIndexedInSplunk()
}

func waitForEventToBeIndexedInSplunk(time_optional ...int) {
	time_sec := 3
	if len(time_optional) > 0 {
		time_sec = time_optional[0]
	}
	time.Sleep(time.Duration(time_sec) * time.Second)
}

func addTextEvent(data string) {
	path := viperConfigVariable("config.PATH_TO_OTEL_COLLECTOR_BIN_DIR")
	f, err := os.OpenFile(path+"/test_file.json",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(data + "\n"); err != nil {
		log.Println(err)
	}
	waitForEventToBeIndexedInSplunk(15)
}

func addJsonTextEvent(data map[string]interface{}) {
	byteArray, _ := json.Marshal(data)
	path := viperConfigVariable("config.PATH_TO_OTEL_COLLECTOR_BIN_DIR")
	f, err := os.OpenFile(path+"/test_file.json",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.Write(byteArray); err != nil {
		log.Println(err)
	}
	if _, err = f.WriteString("\n"); err != nil {
		log.Println(err)
	}
	waitForEventToBeIndexedInSplunk(10)
}
