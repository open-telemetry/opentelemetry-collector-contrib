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
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ecs"
	"go.uber.org/zap"
)

const (
	AwsSdkLevelRetryCount = 3
)

type serviceDiscovery struct {
	svcEcs *ecs.ECS
	svcEc2 *ec2.EC2
	config *Config
}

func (sd *serviceDiscovery) init() {
	region := getAWSRegion(sd.config)
	awsConfig := aws.NewConfig().WithRegion(region).WithMaxRetries(AwsSdkLevelRetryCount)
	session := session.New(awsConfig)
	sd.svcEcs = ecs.New(session, awsConfig)
	sd.svcEc2 = ec2.New(session, awsConfig)
}

// getAWSRegion retrieves the AWS region from the provided config, env var, or EC2 metadata.
func getAWSRegion(cfg *Config) (awsRegion string) {
	awsRegion = cfg.ClusterRegion

	if awsRegion == "" {
		if regionEnv := os.Getenv("AWS_REGION"); regionEnv != "" {
			awsRegion = regionEnv
			cfg.logger.Debug("Cluster region not defined. Fetched region from environment variables", zap.String("region", awsRegion))
		} else {
			if s, err := session.NewSession(); err != nil {
				cfg.logger.Error("Unable to create default session", zap.Error(err))
			} else {
				awsRegion, err = ec2metadata.New(s).Region()
				if err != nil {
					cfg.logger.Error("Unable to retrieve the region from the EC2 instance", zap.Error(err))
				} else {
					cfg.logger.Debug("Fetch region from EC2 metadata", zap.String("region", awsRegion))
				}
			}
		}
	}

	if awsRegion == "" {
		cfg.logger.Error("Cannot fetch region variable from config file, environment variables, or ec2 metadata.")
	}

	return
}
