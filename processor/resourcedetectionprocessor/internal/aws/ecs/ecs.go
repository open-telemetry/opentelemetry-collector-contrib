package ecs

import (
	"context"
	"fmt"
	"go.opentelemetry.io/collector/translator/conventions"
	"log"
	"os"
	"strings"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"go.opentelemetry.io/collector/consumer/pdata"
)

const (
	TypeStr     = "ecs"
	tmde3EnvVar = "ECS_CONTAINER_METADATA_URI"
	tmde4EnvVar = "ECS_CONTAINER_METADATA_URI_V4"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	provider ecsMetadataProvider
}

func NewDetector() (internal.Detector, error) {
	return &Detector{provider: &ecsMetadataProviderImpl{}}, nil
}

// Records metadata retrieved from the ECS Task Metadata Endpoint (TMDE) as resource attributes
// TODO: Replace all attribute fields and enums with values defined in "conventions" once they exist
func (d *Detector) Detect(context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	res.InitEmpty()

	tmde := getTmdeFromEnv()

	// Fail fast if neither env var is present
	if tmde == "" {
		log.Println("No Task Metadata Endpoint environment variable detected, skipping ECS resource detection")
		return res, nil
	}

	tmdeResp, err := d.provider.fetchTaskMetaData(tmde)

	if err != nil || tmdeResp == nil {
		return res, err
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString(conventions.AttributeCloudZone, tmdeResp.AvailabilityZone)
	attr.InsertString("cloud.infrastructure_service", "ECS")
	attr.InsertString("aws.ecs.cluster", tmdeResp.Cluster)
	attr.InsertString("aws.ecs.task.arn", tmdeResp.TaskARN)
	attr.InsertString("aws.ecs.task.family", tmdeResp.Family)

	region, account := parseRegionAndAccount(tmdeResp.TaskARN)
	if account != "" {
		attr.InsertString(conventions.AttributeCloudAccount, account)
	}

	if region != "" {
		attr.InsertString(conventions.AttributeCloudRegion, region)
	}

	// The launch type and log data attributes are only available in v4 of TMDE
	switch lt := strings.ToLower(tmdeResp.LaunchType); lt {
	case "ec2":
		attr.InsertString("aws.ecs.launchtype", "EC2")

	case "fargate":
		attr.InsertString("aws.ecs.launchtype", "Fargate")
	}

	selfMetaData, err := d.provider.fetchContainerMetaData(tmde)

	if err != nil || selfMetaData == nil {
		return res, err
	}

	logAttributes := [4]string{"aws.log.group.names", "aws.log.group.arns", "aws.log.stream.names", "aws.log.stream.arns"}

	for i, attribVal := range getValidLogData(tmdeResp.Containers, selfMetaData, account) {
		if attribVal.Len() > 0 {
			ava := pdata.NewAttributeValueArray()
			ava.SetArrayVal(attribVal)
			attr.Insert(logAttributes[i], ava)
		}
	}

	return res, nil
}

func getTmdeFromEnv() string {
	var tmde string
	if tmde = strings.TrimSpace(os.Getenv(tmde4EnvVar)); tmde == "" {
		tmde = strings.TrimSpace(os.Getenv(tmde3EnvVar))
	}

	return tmde
}

// Parses the AWS Account ID and AWS Region from a task ARN
// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-account-settings.html#ecs-resource-ids
func parseRegionAndAccount(taskARN string) (region string, account string) {
	parts := strings.Split(taskARN, ":")
	if len(parts) >= 5 {
		return parts[3], parts[4]
	}

	return "", ""
}

// Filter out non-normal containers, our own container since we assume the collector is run as a sidecar,
// "init" containers which only run at startup then shutdown (as indicated by the "KnownStatus" attribute),
// containers not using AWS Logs, and those without log group metadata to get the final lists of valid log data
// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4.html#task-metadata-endpoint-v4-response
func getValidLogData(containers []Container, self *Container, account string) [4]pdata.AnyValueArray {
	lgn := pdata.NewAnyValueArray()
	lga := pdata.NewAnyValueArray()
	lsn := pdata.NewAnyValueArray()
	lsa := pdata.NewAnyValueArray()

	for _, container := range containers {
		logData := container.LogOptions
		if container.Type == "NORMAL" &&
			container.KnownStatus == "RUNNING" &&
			container.LogDriver == "awslogs" &&
			self.DockerId != container.DockerId &&
			logData != (LogData{}) {

			lgn.Append(pdata.NewAttributeValueString(logData.LogGroup))
			lga.Append(pdata.NewAttributeValueString(constructLogGroupArn(logData.Region, account, logData.LogGroup)))
			lsn.Append(pdata.NewAttributeValueString(logData.Stream))
			lsa.Append(pdata.NewAttributeValueString(constructLogStreamArn(logData.Region, account, logData.LogGroup, logData.Stream)))
		}
	}

	return [4]pdata.AnyValueArray{lgn, lga, lsn, lsa}
}

func constructLogGroupArn(region, account, group string) string {
	return fmt.Sprintf("arn:aws:logs:%s:%s:log-group:%s", region, account, group)
}

func constructLogStreamArn(region, account, group, stream string) string {
	return fmt.Sprintf("%s:log-stream:%s", constructLogGroupArn(region, account, group), stream)
}
