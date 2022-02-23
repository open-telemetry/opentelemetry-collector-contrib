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

package googlecloudexporter

import (
	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	"google.golang.org/genproto/googleapis/logging/v2"
	"google.golang.org/protobuf/proto"
)

const defaultMaxRequestSize = 10000000

// RequestBuilder is an interface for creating requests to the google logging API
type RequestBuilder interface {
	Build(resourceLogGroups *pdata.Logs) []*logging.WriteLogEntriesRequest
}

// GoogleRequestBuilder builds google cloud logging requests
type GoogleRequestBuilder struct {
	MaxRequestSize int
	ProjectID      string
	EntryBuilder   EntryBuilder
	*zap.SugaredLogger
}

// Build builds a series of google write log entries requests from OTEL logs
func (g *GoogleRequestBuilder) Build(logs *pdata.Logs) []*logging.WriteLogEntriesRequest {
	protoEntries := []*logging.LogEntry{}

	resourceLogGroups := logs.ResourceLogs()

	for i := 0; i < resourceLogGroups.Len(); i++ {
		resourceLogs := resourceLogGroups.At(i)

		instrumentationLibraryLogGroups := resourceLogs.InstrumentationLibraryLogs()
		for j := 0; j < instrumentationLibraryLogGroups.Len(); j++ {
			instrumentationLibraryLogs := instrumentationLibraryLogGroups.At(j)
			logRecords := instrumentationLibraryLogs.LogRecords()
			for k := 0; k < logRecords.Len(); k++ {
				logRecord := logRecords.At(k)
				protoEntry, err := g.EntryBuilder.Build(&logRecord)
				if err != nil {
					g.Errorw("Failed to create protobuf entry. Dropping entry", zap.Any("error", err))
					continue
				}
				protoEntries = append(protoEntries, protoEntry)
			}
		}
	}

	return g.buildRequests(protoEntries)
}

// buildRequests builds a series of requests from the supplied protobuf entries.
// The number of requests created cooresponds to the max request size of the builder.
func (g *GoogleRequestBuilder) buildRequests(entries []*logging.LogEntry) []*logging.WriteLogEntriesRequest {
	request := g.buildRequest(entries)
	size := proto.Size(request)
	if size <= g.MaxRequestSize {
		g.Debugw("Created write request", "size", size, "entries", len(entries))
		return []*logging.WriteLogEntriesRequest{request}
	}

	if len(request.Entries) == 1 {
		g.Errorw("Single entry exceeds max request size. Dropping entry", "size", size)
		return []*logging.WriteLogEntriesRequest{}
	}

	totalEntries := len(request.Entries)
	firstRequest := g.buildRequest([]*logging.LogEntry{})
	firstSize := 0
	index := 0

	for i, entry := range request.Entries {

		firstRequest.Entries = append(firstRequest.Entries, entry)
		firstSize = proto.Size(firstRequest)

		if firstSize > g.MaxRequestSize {
			index = i
			firstRequest.Entries = firstRequest.Entries[:index]
			break
		}
	}

	secondEntries := request.Entries[index:totalEntries]
	secondRequests := g.buildRequests(secondEntries)

	return append([]*logging.WriteLogEntriesRequest{firstRequest}, secondRequests...)
}

// buildRequest builds a request from the supplied protobuf entries
func (g *GoogleRequestBuilder) buildRequest(entries []*logging.LogEntry) *logging.WriteLogEntriesRequest {
	return &logging.WriteLogEntriesRequest{
		LogName:  createLogName(g.ProjectID, "default"),
		Resource: g.defaultResource(),
		Entries:  entries,
	}
}

// defaultResources creates a default global resource from the operator's projectID
func (g *GoogleRequestBuilder) defaultResource() *monitoredres.MonitoredResource {
	return &monitoredres.MonitoredResource{
		Type: "global",
		Labels: map[string]string{
			"project_id": g.ProjectID,
		},
	}
}
