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

package ec2

import (
	"context"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	TypeStr   = "ec2"
	tagPrefix = "ec2.tag."
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	metadataProvider metadataProvider
	cfg              Config
}

func NewDetector(_ component.ProcessorCreateParams, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	err = validateRegexes(cfg)
	if err != nil {
		return nil, err
	}
	return &Detector{metadataProvider: newMetadataClient(sess), cfg: cfg}, nil
}

func (d *Detector) Detect(ctx context.Context) (pdata.Resource, error) {
	res := pdata.NewResource()
	if !d.metadataProvider.available(ctx) {
		return res, nil
	}

	meta, err := d.metadataProvider.get(ctx)
	if err != nil {
		return res, fmt.Errorf("failed getting identity document: %w", err)
	}

	hostname, err := d.metadataProvider.hostname(ctx)
	if err != nil {
		return res, fmt.Errorf("failed getting hostname: %w", err)
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString("cloud.infrastructure_service", "EC2")
	attr.InsertString(conventions.AttributeCloudRegion, meta.Region)
	attr.InsertString(conventions.AttributeCloudAccount, meta.AccountID)
	attr.InsertString(conventions.AttributeCloudZone, meta.AvailabilityZone)
	attr.InsertString(conventions.AttributeHostID, meta.InstanceID)
	attr.InsertString(conventions.AttributeHostImageID, meta.ImageID)
	attr.InsertString(conventions.AttributeHostType, meta.InstanceType)
	attr.InsertString(conventions.AttributeHostName, hostname)

	tags, err := connectAndFetchEc2Tags(meta.Region, meta.InstanceID, d.cfg)
	if err != nil {
		return res, fmt.Errorf("failed fetching ec2 instance tags: %w", err)
	}

	for key, val := range tags {
		attr.InsertString(tagPrefix+key, val)
	}

	return res, nil
}

func connectAndFetchEc2Tags(region string, instanceID string, cfg Config) (map[string]string, error) {
	if len(cfg.Tags) == 0 {
		return nil, nil
	}

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return nil, err
	}
	e := ec2.New(sess)

	return fetchEC2Tags(e, instanceID, cfg)
}

func fetchEC2Tags(svc ec2iface.EC2API, instanceID string, cfg Config) (map[string]string, error) {
	ec2Tags, err := svc.DescribeTags(&ec2.DescribeTagsInput{
		Filters: []*ec2.Filter{{
			Name: aws.String("resource-id"),
			Values: []*string{
				aws.String(instanceID),
			},
		}},
	})
	if err != nil {
		return nil, err
	}
	tags := make(map[string]string)
	for _, tag := range ec2Tags.Tags {
		matched := regexArrayMatch(cfg.Tags, *tag.Key)
		if matched {
			tags[*tag.Key] = *tag.Value
		}
	}
	return tags, nil
}

func validateRegexes(cfg Config) error {
	for _, elem := range cfg.Tags {
		_, err := regexp.Compile(elem)
		if err != nil {
			return err
		}
	}
	return nil
}

func regexArrayMatch(arr []string, val string) bool {
	for _, elem := range arr {
		matched, _ := regexp.MatchString(elem, val)
		if matched {
			return true
		}
	}
	return false
}
