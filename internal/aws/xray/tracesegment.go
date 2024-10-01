// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsxray // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray"

import (
	"encoding/json"
	"errors"
	"fmt"
)

type CauseType int

const (
	// CauseTypeExceptionID indicates that the type of the `cause`
	// field is a string
	CauseTypeExceptionID CauseType = iota + 1
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
	Service     *ServiceData   `json:"service,omitempty"`
	Origin      *string        `json:"origin,omitempty"`
	User        *string        `json:"user,omitempty"`
	ResourceARN *string        `json:"resource_arn,omitempty"`
	Links       []SpanLinkData `json:"links,omitempty"`

	// Optional fields for both Segment and subsegments
	TraceID     *string                   `json:"trace_id,omitempty"`
	EndTime     *float64                  `json:"end_time,omitempty"`
	InProgress  *bool                     `json:"in_progress,omitempty"`
	HTTP        *HTTPData                 `json:"http,omitempty"`
	Fault       *bool                     `json:"fault,omitempty"`
	Error       *bool                     `json:"error,omitempty"`
	Throttle    *bool                     `json:"throttle,omitempty"`
	Cause       *CauseData                `json:"cause,omitempty"`
	AWS         *AWSData                  `json:"aws,omitempty"`
	Annotations map[string]any            `json:"annotations,omitempty"`
	Metadata    map[string]map[string]any `json:"metadata,omitempty"`
	Subsegments []Segment                 `json:"subsegments,omitempty"`

	// (for both embedded and independent) subsegment-only (optional) fields.
	// Please refer to https://docs.aws.amazon.com/xray/latest/devguide/xray-api-segmentdocuments.html#api-segmentdocuments-subsegments
	// for more information on subsegment.
	Namespace    *string  `json:"namespace,omitempty"`
	ParentID     *string  `json:"parent_id,omitempty"`
	Type         *string  `json:"type,omitempty"`
	PrecursorIDs []string `json:"precursor_ids,omitempty"`
	Traced       *bool    `json:"traced,omitempty"`
	SQL          *SQLData `json:"sql,omitempty"`
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
	Beanstalk *BeanstalkMetadata `json:"elastic_beanstalk,omitempty"`
	CWLogs    []LogGroupMetadata `json:"cloudwatch_logs,omitempty"`
	ECS       *ECSMetadata       `json:"ecs,omitempty"`
	EC2       *EC2Metadata       `json:"ec2,omitempty"`
	EKS       *EKSMetadata       `json:"eks,omitempty"`
	XRay      *XRayMetaData      `json:"xray,omitempty"`

	// For both segment and subsegments
	AccountID    *string  `json:"account_id,omitempty"`
	Operation    *string  `json:"operation,omitempty"`
	RemoteRegion *string  `json:"region,omitempty"`
	RequestID    *string  `json:"request_id,omitempty"`
	QueueURL     *string  `json:"queue_url,omitempty"`
	TableName    *string  `json:"table_name,omitempty"`
	TableNames   []string `json:"table_names,omitempty"`
	Retries      *int64   `json:"retries,omitempty"`
}

// EC2Metadata represents the EC2 metadata field
type EC2Metadata struct {
	InstanceID       *string `json:"instance_id"`
	AvailabilityZone *string `json:"availability_zone"`
	InstanceSize     *string `json:"instance_size"`
	AmiID            *string `json:"ami_id"`
}

// ECSMetadata represents the ECS metadata field. All must be omitempty b/c they come from two different detectors:
// Docker and ECS, so it's possible one is present and not the other
type ECSMetadata struct {
	ContainerName    *string `json:"container,omitempty"`
	ContainerID      *string `json:"container_id,omitempty"`
	TaskArn          *string `json:"task_arn,omitempty"`
	TaskFamily       *string `json:"task_family,omitempty"`
	ClusterArn       *string `json:"cluster_arn,omitempty"`
	ContainerArn     *string `json:"container_arn,omitempty"`
	AvailabilityZone *string `json:"availability_zone,omitempty"`
	LaunchType       *string `json:"launch_type,omitempty"`
}

// BeanstalkMetadata represents the Elastic Beanstalk environment metadata field
type BeanstalkMetadata struct {
	Environment  *string `json:"environment_name"`
	VersionLabel *string `json:"version_label"`
	DeploymentID *int64  `json:"deployment_id"`
}

// EKSMetadata represents the EKS metadata field
type EKSMetadata struct {
	ClusterName *string `json:"cluster_name"`
	Pod         *string `json:"pod"`
	ContainerID *string `json:"container_id"`
}

// LogGroupMetadata represents a single CloudWatch Log Group
type LogGroupMetadata struct {
	LogGroup *string `json:"log_group"`
	Arn      *string `json:"arn,omitempty"`
}

// CauseData is the container that contains the `cause` field
type CauseData struct {
	Type CauseType `json:"-"`
	// it will contain one of ExceptionID or (WorkingDirectory, Paths, Exceptions)
	ExceptionID *string `json:"-"`

	CauseObject
}

type CauseObject struct {
	WorkingDirectory *string     `json:"working_directory,omitempty"`
	Paths            []string    `json:"paths,omitempty"`
	Exceptions       []Exception `json:"exceptions,omitempty"`
}

// UnmarshalJSON is the custom unmarshaller for the cause field
func (c *CauseData) UnmarshalJSON(data []byte) error {
	err := json.Unmarshal(data, &c.CauseObject)
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
	ID        *string      `json:"id,omitempty"`
	Message   *string      `json:"message,omitempty"`
	Type      *string      `json:"type,omitempty"`
	Remote    *bool        `json:"remote,omitempty"`
	Truncated *int64       `json:"truncated,omitempty"`
	Skipped   *int64       `json:"skipped,omitempty"`
	Cause     *string      `json:"cause,omitempty"`
	Stack     []StackFrame `json:"stack,omitempty"`
}

// StackFrame represents a frame in the stack when an exception occurred
type StackFrame struct {
	Path  *string `json:"path,omitempty"`
	Line  *int    `json:"line,omitempty"`
	Label *string `json:"label,omitempty"`
}

// HTTPData provides the shape for unmarshalling request and response fields.
type HTTPData struct {
	Request  *RequestData  `json:"request,omitempty"`
	Response *ResponseData `json:"response,omitempty"`
}

// RequestData provides the shape for unmarshalling the request field.
type RequestData struct {
	// Available in segment
	XForwardedFor *bool `json:"x_forwarded_for,omitempty"`

	// Available in both segment and subsegments
	Method    *string `json:"method,omitempty"`
	URL       *string `json:"url,omitempty"`
	UserAgent *string `json:"user_agent,omitempty"`
	ClientIP  *string `json:"client_ip,omitempty"`
}

// ResponseData provides the shape for unmarshalling the response field.
type ResponseData struct {
	Status        *int64 `json:"status,omitempty"`
	ContentLength any    `json:"content_length,omitempty"`
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

// XRayMetaData provides the shape for unmarshalling the xray field
type XRayMetaData struct {
	SDK                 *string `json:"sdk,omitempty"`
	SDKVersion          *string `json:"sdk_version,omitempty"`
	AutoInstrumentation *bool   `json:"auto_instrumentation"`
}

// SQLData provides the shape for unmarshalling the sql field.
type SQLData struct {
	ConnectionString *string `json:"connection_string,omitempty"`
	URL              *string `json:"url,omitempty"` // protocol://host[:port]/database
	SanitizedQuery   *string `json:"sanitized_query,omitempty"`
	DatabaseType     *string `json:"database_type,omitempty"`
	DatabaseVersion  *string `json:"database_version,omitempty"`
	DriverVersion    *string `json:"driver_version,omitempty"`
	User             *string `json:"user,omitempty"`
	Preparation      *string `json:"preparation,omitempty"` // "statement" / "call"
}

// ServiceData provides the shape for unmarshalling the service field.
type ServiceData struct {
	Version         *string `json:"version,omitempty"`
	CompilerVersion *string `json:"compiler_version,omitempty"`
	Compiler        *string `json:"compiler,omitempty"`
}

// SpanLinkData provides the shape for unmarshalling the span links in the span link field.
type SpanLinkData struct {
	TraceID    *string        `json:"trace_id"`
	SpanID     *string        `json:"id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}
