// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
