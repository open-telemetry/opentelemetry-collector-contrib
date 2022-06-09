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

package model // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver/internal/model"

// Alert represents an alert. It uses the format defined by mongodb atlas.
// See: https://www.mongodb.com/docs/atlas/reference/api/alerts-get-alert/#response-elements
type Alert struct {
	Created                 string  `json:"created"`
	AlertConfigID           string  `json:"alertConfigId"`
	GroupID                 string  `json:"groupId"`
	EventType               string  `json:"eventTypeName"`
	ID                      string  `json:"id"`
	HumanReadable           string  `json:"humanReadable"`
	Updated                 string  `json:"updated"`
	Status                  string  `json:"status"`
	ReplicaSetName          *string `json:"replicaSetName"`
	MetricName              *string `json:"metricName"`
	TypeName                *string `json:"typeName"`
	ClusterName             *string `json:"clusterName"`
	UserAlias               *string `json:"userAlias"`
	LastNotified            *string `json:"lastNotified"`
	Resolved                *string `json:"resolved"`
	AcknowledgementComment  *string `json:"acknowledgementComment"`
	AcknowledgementUsername *string `json:"acknowledgingUsername"`
	AcknowledgedUntil       *string `json:"acknowledgedUntil"`
	CurrentValue            *struct {
		Number float64 `json:"number"`
		Units  string  `json:"units"`
	} `json:"currentValue"`
	HostNameAndPort *string `json:"hostnameAndPort"`
}
