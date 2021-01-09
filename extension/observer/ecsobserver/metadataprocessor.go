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

package ecsobserver

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/hashicorp/golang-lru/simplelru"
	"go.uber.org/zap"
)

const (
	// ECS Service Quota: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/service-quotas.html
	ec2metadataCacheSize = 2000
	batchSize            = 100
)

// MetadataProcessor adds EC2 metadata for ECS Clusters.
type MetadataProcessor struct {
	svcEc2 *ec2.EC2
	svcEcs *ecs.ECS

	ec2Cache *simplelru.LRU
	logger   *zap.Logger
}

func (p *MetadataProcessor) ProcessorName() string {
	return "MetadataProcessor"
}

// Process retrieves EC2 metadata for ECS container instances.
func (p *MetadataProcessor) Process(clusterName string, taskList []*ECSTask) ([]*ECSTask, error) {
	arnBatches := make([][]string, 0)
	currBatch := make([]string, 0, batchSize)

	for _, task := range taskList {
		if aws.StringValue(task.Task.LaunchType) != ecs.LaunchTypeEc2 {
			continue
		}

		ciArn := aws.StringValue(task.Task.ContainerInstanceArn)
		if ciArn == "" {
			continue
		}

		if res, ok := p.ec2Cache.Get(ciArn); ok {
			// Try retrieving from cache
			task.EC2Info = res.(*EC2MetaData)
		} else {
			// Save for querying via API
			if len(currBatch) >= batchSize {
				arnBatches = append(arnBatches, currBatch)
				currBatch = make([]string, 0, batchSize)
			}
			currBatch = append(currBatch, ciArn)
		}
	}

	if len(currBatch) > 0 {
		arnBatches = append(arnBatches, currBatch)
	}

	if len(arnBatches) == 0 {
		return taskList, nil
	}

	// Retrieve EC2 metadata from ARNs
	ec2MetadataMap := make(map[string]*EC2MetaData)
	for _, batch := range arnBatches {
		err := p.getEC2Metadata(clusterName, batch, ec2MetadataMap)
		if err != nil {
			return taskList, err
		}
	}

	// Assign metadata to task
	for _, task := range taskList {
		ciArn := aws.StringValue(task.Task.ContainerInstanceArn)
		if metadata, ok := ec2MetadataMap[ciArn]; ok {
			task.EC2Info = metadata
		}
	}

	return taskList, nil
}

func (p *MetadataProcessor) getEC2Metadata(clusterName string, arns []string, ec2MetadataMap map[string]*EC2MetaData) error {
	// Get the ECS Instances from ARNs
	ecsInput := &ecs.DescribeContainerInstancesInput{
		Cluster:            &clusterName,
		ContainerInstances: aws.StringSlice(arns),
	}
	resp, ecsErr := p.svcEcs.DescribeContainerInstances(ecsInput)
	if ecsErr != nil {
		return fmt.Errorf("failed to DescribeContainerInstances. Error: %s", ecsErr.Error())
	}

	for _, f := range resp.Failures {
		p.logger.Debug(
			"DescribeContainerInstances Failure",
			zap.String("ARN", *f.Arn),
			zap.String("Reason", *f.Reason),
			zap.String("Detail", *f.Detail),
		)
	}

	ec2Ids := make([]*string, 0, len(arns))
	idToArnMap := make(map[string]*string)

	// Retrieve EC2 IDs from ECS container instances
	for _, ci := range resp.ContainerInstances {
		ec2Id := ci.Ec2InstanceId
		ciArn := ci.ContainerInstanceArn
		if ec2Id != nil && ciArn != nil {
			ec2Ids = append(ec2Ids, ec2Id)
			idToArnMap[aws.StringValue(ec2Id)] = ciArn
		}
	}

	// Get the EC2 Instances from EC2 IDs
	ec2input := &ec2.DescribeInstancesInput{InstanceIds: ec2Ids}
	for {
		ec2resp, ec2err := p.svcEc2.DescribeInstances(ec2input)
		if ec2err != nil {
			return fmt.Errorf("failed to DescribeInstances. Error: %s", ec2err.Error())
		}

		for _, rsv := range ec2resp.Reservations {
			for _, instance := range rsv.Instances {
				ec2Id := aws.StringValue(instance.InstanceId)
				if ec2Id == "" {
					continue
				}
				ciArnPtr, ok := idToArnMap[ec2Id]
				if !ok {
					continue
				}
				ciArn := aws.StringValue(ciArnPtr)
				metadata := &EC2MetaData{
					ContainerInstanceID: ciArn,
					ECInstanceID:        ec2Id,
					PrivateIP:           aws.StringValue(instance.PrivateIpAddress),
					InstanceType:        aws.StringValue(instance.InstanceType),
					VpcID:               aws.StringValue(instance.VpcId),
					SubnetID:            aws.StringValue(instance.SubnetId),
				}
				ec2MetadataMap[ciArn] = metadata
				p.ec2Cache.Add(ciArn, metadata)
			}
		}

		if ec2resp.NextToken == nil {
			break
		}
		ec2input.NextToken = ec2resp.NextToken
	}

	return nil
}

func NewMetadataProcessor(ecs *ecs.ECS, ec2 *ec2.EC2, logger *zap.Logger) *MetadataProcessor {
	// Initiate the container instance metadata LRU caching
	lru, _ := simplelru.NewLRU(ec2metadataCacheSize, nil)

	return &MetadataProcessor{
		svcEcs:   ecs,
		svcEc2:   ec2,
		ec2Cache: lru,
		logger:   logger,
	}
}
