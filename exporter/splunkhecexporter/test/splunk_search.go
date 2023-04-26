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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

func checkEventsFromSplunk(searchQuery string, startTime string, end_time_optional ...string) []interface{} {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Splunk Search: checking events in Splunk --")
	baseURL := "https://" + getSplunkHost() + ":" + getSplunkPort()
	user := getSplunkUser()
	password := getSplunkPassword()
	endTime := "now"
	if len(end_time_optional) > 0 {
		endTime = end_time_optional[0]
	}
	// post search
	jobID := postSearchRequest(user, password, baseURL, searchQuery, startTime, endTime)
	// wait for search status done == true
	for i := 0; i < 20; i++ { // limit loop - not allowing infinite looping
		fmt.Println("Checking Search Status ...")
		isDone := checkSearchJobStatusCode(user, password, baseURL, jobID)
		if isDone == true {
			break
		}
		time.Sleep(1 * time.Second)
	}
	// get events
	results := getSplunkSearchResults(user, password, baseURL, jobID)
	return results
}

func getSplunkSearchResults(user string, password string, baseURL string, jobID string) []interface{} {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	eventURL := fmt.Sprintf("%s/services/search/jobs/%s/events?output_mode=json", baseURL, jobID)
	logger.Println("URL: " + eventURL)
	reqEvents, err := http.NewRequest("GET", eventURL, nil)
	if err != nil {
		panic(err)
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	reqEvents.SetBasicAuth(user, password)
	respEvents, err := client.Do(reqEvents)
	if err != nil {
		panic(err)
	}
	defer respEvents.Body.Close()
	logger.Println("Send Request: Get query status code: " + strconv.Itoa(respEvents.StatusCode))

	bodyEvents, err := ioutil.ReadAll(respEvents.Body)
	if err != nil {
		panic(err)
	}

	var jsonResponseEvents map[string]interface{}
	err = json.Unmarshal(bodyEvents, &jsonResponseEvents)
	if err != nil {
		panic(err)
	}

	logger.Println("json Response Events --->")
	logger.Println(jsonResponseEvents)
	results := jsonResponseEvents["results"].([]interface{})
	// logger.Println(results)
	return results
}

func checkSearchJobStatusCode(user string, password string, baseURL string, jobID string) interface{} {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	checkEventURL := baseURL + "/services/search/jobs/" + jobID + "?output_mode=json"
	logger.Println("URL: " + checkEventURL)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	checkReqEvents, err := http.NewRequest("GET", checkEventURL, nil)
	if err != nil {
		panic(err)
	}
	checkReqEvents.SetBasicAuth(user, password)
	checkResp, err := client.Do(checkReqEvents)
	if err != nil {
		panic(err)
	}
	defer checkResp.Body.Close()
	logger.Println("Send Request: Check query status code: " + strconv.Itoa(checkResp.StatusCode))
	check_body, err := ioutil.ReadAll(checkResp.Body)
	if err != nil {
		panic(err)
	}
	var checkJsonResponse map[string]interface{}
	err = json.Unmarshal(check_body, &checkJsonResponse)
	if err != nil {
		panic(err)
	}
	// logger.Println(checkJsonResponse) // debug
	// Print isDone field from response
	isDone := checkJsonResponse["entry"].([]interface{})[0].(map[string]interface{})["content"].(map[string]interface{})["isDone"]
	logger.Printf("Is Splunk Search compleated [isDone flag]: %v\n", isDone)
	return isDone
}
func postSearchRequest(user string, password string, baseURL string, searchQuery string, startTime string, endTime string) string {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	searchURL := fmt.Sprintf("%s/services/search/jobs?output_mode=json", baseURL)
	query := "search " + searchQuery
	logger.Println("Search query: " + query)
	data := url.Values{}
	data.Set("search", query)
	data.Set("earliest_time", startTime)
	data.Set("latest_time", endTime)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("POST", searchURL, strings.NewReader(data.Encode()))
	if err != nil {
		logger.Printf("Error while preparing POST request")
		panic(err)
	}
	req.SetBasicAuth(user, password)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		logger.Printf("Error while executing Http POST request")
		panic(err)
	}
	defer resp.Body.Close()
	logger.Println("Send Request: Post query status code: " + strconv.Itoa(resp.StatusCode))

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		panic(err)
	}
	logger.Println(jsonResponse) // debug
	return jsonResponse["sid"].(string)
}

func checkMetricsFromSplunk(index string, metricName string) []interface{} {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	logger.Println("-->> Splunk Search: checking metrics in Splunk --")
	url := "https://" + getSplunkHost() + ":" + getSplunkPort()
	startTime := "-1d@d"
	endTime := "now"
	user := getSplunkUser()
	password := getSplunkPassword()

	apiURL := fmt.Sprintf("%s/services/catalog/metricstore/dimensions/host/values?filter=index%%3d%s&metric_name=%s&earliest=%s&latest=%s&output_mode=json", url, index, metricName, startTime, endTime)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		panic(err)
	}
	req.SetBasicAuth(user, password)

	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		panic(err)
	}

	events := data["entry"].([]interface{})
	// logger.Println(events) // debug

	return events
}
