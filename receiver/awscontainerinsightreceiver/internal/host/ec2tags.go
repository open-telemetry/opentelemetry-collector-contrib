// Copyright  OpenTelemetry Authors
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

package host

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

const (
	ClusterNameKey          = "container-insight-eks-cluster-name"
	ClusterNameTagKeyPrefix = "kubernetes.io/cluster/"
	AutoScalingGroupNameTag = "aws:autoscaling:groupName"
)

type EC2Tags struct {
	refreshInterval      time.Duration
	instanceID           string
	clusterName          string
	autoScalingGroupName string
	logger               *zap.Logger
	shutdownC            chan bool
}

func NewEC2Tags(instanceID string, refreshInterval time.Duration, logger *zap.Logger) *EC2Tags {
	if instanceID == "" {
		return nil
	}

	et := &EC2Tags{
		instanceID:      instanceID,
		refreshInterval: refreshInterval,
		shutdownC:       make(chan bool),
		logger:          logger,
	}

	et.refresh()

	shouldRefresh := func() bool {
		//stop once we get the cluster name
		return et.clusterName == ""
	}

	go refreshUntil(et.refresh, et.refreshInterval, shouldRefresh, et.shutdownC)

	return et
}

func (et *EC2Tags) fetchEC2Tags() map[string]string {
	et.logger.Info("Fetch ec2 tags to detect cluster name and auto scaling group name")
	tags := make(map[string]string)
	//add some sleep jitter to prevent a large number of receivers calling the ec2 api at the same time
	time.Sleep(hostJitter(3 * time.Second))

	sess, err := session.NewSession(&aws.Config{})
	if err != nil {
		et.logger.Warn("Fail to set up session to call ec2 api", zap.Error(err))
	}

	tagFilters := []*ec2.Filter{
		{
			Name:   aws.String("resource-type"),
			Values: aws.StringSlice([]string{"instance"}),
		},
		{
			Name:   aws.String("resource-id"),
			Values: aws.StringSlice([]string{et.instanceID}),
		},
	}

	client := ec2.New(sess)
	input := &ec2.DescribeTagsInput{
		Filters: tagFilters,
	}

	for {
		result, err := client.DescribeTags(input)
		if err != nil {
			et.logger.Warn("Fail to call ec2 DescribeTags", zap.Error(err))
			break
		}

		for _, tag := range result.Tags {
			key := *tag.Key
			tags[key] = *tag.Value
			if strings.HasPrefix(key, ClusterNameTagKeyPrefix) && *tag.Value == "owned" {
				tags[ClusterNameKey] = key[len(ClusterNameTagKeyPrefix):]
			}
		}

		if result.NextToken == nil {
			break
		}
		input.SetNextToken(*result.NextToken)
	}

	return tags
}

func (et *EC2Tags) GetClusterName() string {
	return et.clusterName
}

func (et *EC2Tags) GetAutoScalingGroupName() string {
	return et.autoScalingGroupName
}

func (et *EC2Tags) refresh() {
	tags := et.fetchEC2Tags()
	et.clusterName = tags[ClusterNameKey]
	et.autoScalingGroupName = tags[AutoScalingGroupNameTag]
}

func (et *EC2Tags) Shutdown() {
	close(et.shutdownC)
}
