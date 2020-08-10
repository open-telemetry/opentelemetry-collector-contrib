// Copyright The OpenTelemetry Authors
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

package tracesegment

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// TypeStr is the type and ingest format of this receiver
	TypeStr = "awsxray"
)

// Header stores header of trace segment.
type Header struct {
	Format  string `json:"format"`
	Version int    `json:"version"`
}

// IsValid validates Header.
func (t Header) IsValid() bool {
	return strings.EqualFold(t.Format, "json") && t.Version == 1
}

type causeType int

const (
	// CauseTypeExceptionID indicates that the type of the `cause`
	// field is a string
	CauseTypeExceptionID causeType = iota + 1
	// CauseTypeObject indicates that the type of the `cause`
	// field is an object
	CauseTypeObject
)

// Segment schema is documented in xray-segmentdocument-schema-v1.0.0 listed
// on https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html
type Segment struct {
	// Required fields for both segment and subsegments
	Name      *string  `json:"name"`
	ID        *string  `json:"id"`
	StartTime *float64 `json:"start_time"`

	// Segment-only optional fields
	Service     *ServiceData `json:"service"`
	Origin      *string      `json:"origin"`
	User        *string      `json:"user"`
	ResourceARN *string      `json:"resource_arn"`

	// Optional fields for both Segment and subsegments
	TraceID     *string                           `json:"trace_id"`
	EndTime     *float64                          `json:"end_time"`
	InProgress  *bool                             `json:"in_progress"`
	HTTP        *HTTPData                         `json:"http"`
	Fault       *bool                             `json:"fault"`
	Error       *bool                             `json:"error"`
	Throttle    *bool                             `json:"throttle"`
	Cause       *CauseData                        `json:"cause"`
	AWS         *AWSData                          `json:"aws"`
	Annotations map[string]interface{}            `json:"annotations"`
	Metadata    map[string]map[string]interface{} `json:"metadata"`
	Subsegments []Segment                         `json:"subsegments"`

	// (for both embedded and independent) subsegment-only (optional) fields.
	// Please refer to https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-subsegments
	// for more information on subsegment.
	Namespace    *string  `json:"namespace"`
	ParentID     *string  `json:"parent_id"`
	Type         *string  `json:"type"`
	PrecursorIDs []string `json:"precursor_ids"`
	Traced       *bool    `json:"traced"`
	SQL          *SQLData `json:"sql"`
}

// Validate checks whether the segment is valid or not
func (s *Segment) Validate() error {
	if s.Name == nil {
		return errors.New(`segment "name" can not be nil`)
	}

	if s.ID == nil {
		return errors.New(`segment "id" can not be nil`)
	}

	if s.StartTime == nil {
		return errors.New(`segment "start_time" can not be nil`)
	}

	// it's ok for embedded subsegments to not have trace_id
	// but the root segment and independent subsegments must all
	// have trace_id.
	if s.TraceID == nil {
		return errors.New(`segment "trace_id" can not be nil`)
	}

	return nil
}

// AWSData represents the aws resource that this segment
// originates from
type AWSData struct {
	// Segment-only
	Beanstalk *BeanstalkMetadata `json:"elastic_beanstalk"`
	ECS       *ECSMetadata       `json:"ecs"`
	EC2       *EC2Metadata       `json:"ec2"`
	XRay      *XRayMetaData      `json:"xray"`

	// For both segment and subsegments
	AccountID    *string `json:"account_id"`
	Operation    *string `json:"operation"`
	RemoteRegion *string `json:"region"`
	RequestID    *string `json:"request_id"`
	QueueURL     *string `json:"queue_url"`
	TableName    *string `json:"table_name"`
	Retries      *int    `json:"retries"`
}

// EC2Metadata represents the EC2 metadata field
type EC2Metadata struct {
	InstanceID       *string `json:"instance_id"`
	AvailabilityZone *string `json:"availability_zone"`
	InstanceSize     *string `json:"instance_size"`
	AmiID            *string `json:"ami_id"`
}

// ECSMetadata represents the ECS metadata field
type ECSMetadata struct {
	ContainerName *string `json:"container"`
}

// BeanstalkMetadata represents the Elastic Beanstalk environment metadata field
type BeanstalkMetadata struct {
	Environment  *string `json:"environment_name"`
	VersionLabel *string `json:"version_label"`
	DeploymentID *int64  `json:"deployment_id"`
}

// CauseData is the container that contains the `cause` field
type CauseData struct {
	Type causeType `json:"-"`
	// it will contain one of ExceptionID or (WorkingDirectory, Paths, Exceptions)
	ExceptionID *string `json:"-"`

	causeObject
}

type causeObject struct {
	WorkingDirectory *string     `json:"working_directory"`
	Paths            []string    `json:"paths"`
	Exceptions       []Exception `json:"exceptions"`
}

// UnmarshalJSON is the custom unmarshaller for the cause field
func (c *CauseData) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &c.causeObject)
	if err == nil {
		c.Type = CauseTypeObject
		return nil
	}
	rawStr := string(data)
	if len(rawStr) > 0 && (rawStr[0] != '"' || rawStr[len(rawStr)-1] != '"') {
		return fmt.Errorf("the value assigned to the `cause` field does not appear to be a string: %v", data)
	}
	exceptionID := rawStr[1 : len(rawStr)-1]

	c.Type = CauseTypeExceptionID
	c.ExceptionID = &exceptionID
	return nil
}

// Exception represents an exception occurred
type Exception struct {
	ID        *string      `json:"id"`
	Message   *string      `json:"message"`
	Type      *string      `json:"type"`
	Remote    *bool        `json:"remote"`
	Truncated *int         `json:"truncated"`
	Skipped   *int         `json:"skipped"`
	Cause     *string      `json:"cause"`
	Stack     []StackFrame `json:"stack"`
}

// StackFrame represents a frame in the stack when an exception occurred
type StackFrame struct {
	Path  *string `json:"path"`
	Line  *int    `json:"line"`
	Label *string `json:"label"`
}

// HTTPData provides the shape for unmarshalling request and response fields.
type HTTPData struct {
	Request  *RequestData  `json:"request"`
	Response *ResponseData `json:"response"`
}

// RequestData provides the shape for unmarshalling the request field.
type RequestData struct {
	// Available in segment
	XForwardedFor *bool `json:"x_forwarded_for"`

	// Available in both segment and subsegments
	Method    *string `json:"method"`
	URL       *string `json:"url"`
	UserAgent *string `json:"user_agent"`
	ClientIP  *string `json:"client_ip"`
}

// ResponseData provides the shape for unmarshalling the response field.
type ResponseData struct {
	Status        *int `json:"status"`
	ContentLength *int `json:"content_length"`
}

// ECSData provides the shape for unmarshalling the ecs field.
type ECSData struct {
	Container *string `json:"container"`
}

// EC2Data provides the shape for unmarshalling the ec2 field.
type EC2Data struct {
	InstanceID       *string `json:"instance_id"`
	AvailabilityZone *string `json:"availability_zone"`
}

// ElasticBeanstalkData provides the shape for unmarshalling the elastic_beanstalk field.
type ElasticBeanstalkData struct {
	EnvironmentName *string `json:"environment_name"`
	VersionLabel    *string `json:"version_label"`
	DeploymentID    *int    `json:"deployment_id"`
}

// TracingData provides the shape for unmarshalling the tracing data.
type TracingData struct {
	SDK *string `json:"sdk"`
}

// XRayMetaData provides the shape for unmarshalling the xray field
type XRayMetaData struct {
	SDKVersion *string `json:"sdk_version"`
	SDK        *string `json:"sdk"`
}

// SQLData provides the shape for unmarshalling the sql field.
type SQLData struct {
	ConnectionString *string `json:"connection_string"`
	URL              *string `json:"url"` // protocol://host[:port]/database
	SanitizedQuery   *string `json:"sanitized_query"`
	DatabaseType     *string `json:"database_type"`
	DatabaseVersion  *string `json:"database_version"`
	DriverVersion    *string `json:"driver_version"`
	User             *string `json:"user"`
	Preparation      *string `json:"preparation"` // "statement" / "call"
}

// ServiceData provides the shape for unmarshalling the service field.
type ServiceData struct {
	Version         *string `json:"version"`
	CompilerVersion *string `json:"compiler_version"`
	Compiler        *string `json:"compiler"`
}
