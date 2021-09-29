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
	"go.opentelemetry.io/collector/model/pdata"
	conventions "go.opentelemetry.io/collector/model/semconv/v1.5.0"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

const (
	// TypeStr is type of detector.
	TypeStr   = "ec2"
	tagPrefix = "ec2.tag."
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	metadataProvider metadataProvider
	tagKeyRegexes    []*regexp.Regexp
}

func NewDetector(_ component.ProcessorCreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	sess, err := session.NewSession()
	if err != nil {
		return nil, err
	}
	tagKeyRegexes, err := compileRegexes(cfg)
	if err != nil {
		return nil, err
	}
	return &Detector{metadataProvider: newMetadataClient(sess), tagKeyRegexes: tagKeyRegexes}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pdata.Resource, schemaURL string, err error) {
	res := pdata.NewResource()
	if !d.metadataProvider.available(ctx) {
		return res, "", nil
	}

	meta, err := d.metadataProvider.get(ctx)
	if err != nil {
		return res, "", fmt.Errorf("failed getting identity document: %w", err)
	}

	hostname, err := d.metadataProvider.hostname(ctx)
	if err != nil {
		return res, "", fmt.Errorf("failed getting hostname: %w", err)
	}

	attr := res.Attributes()
	attr.InsertString(conventions.AttributeCloudProvider, conventions.AttributeCloudProviderAWS)
	attr.InsertString(conventions.AttributeCloudPlatform, conventions.AttributeCloudPlatformAWSEC2)
	attr.InsertString(conventions.AttributeCloudRegion, meta.Region)
	attr.InsertString(conventions.AttributeCloudAccountID, meta.AccountID)
	attr.InsertString(conventions.AttributeCloudAvailabilityZone, meta.AvailabilityZone)
	attr.InsertString(conventions.AttributeHostID, meta.InstanceID)
	attr.InsertString(conventions.AttributeHostImageID, meta.ImageID)
	attr.InsertString(conventions.AttributeHostType, meta.InstanceType)
	attr.InsertString(conventions.AttributeHostName, hostname)

	if len(d.tagKeyRegexes) != 0 {
		tags, err := connectAndFetchEc2Tags(meta.Region, meta.InstanceID, d.tagKeyRegexes)
		if err != nil {
			return res, "", fmt.Errorf("failed fetching ec2 instance tags: %w", err)
		}
		for key, val := range tags {
			attr.InsertString(tagPrefix+key, val)
		}
	}

	return res, conventions.SchemaURL, nil
}

func connectAndFetchEc2Tags(region string, instanceID string, tagKeyRegexes []*regexp.Regexp) (map[string]string, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region)},
	)
	if err != nil {
		return nil, err
	}
	e := ec2.New(sess)

	return fetchEC2Tags(e, instanceID, tagKeyRegexes)
}

func fetchEC2Tags(svc ec2iface.EC2API, instanceID string, tagKeyRegexes []*regexp.Regexp) (map[string]string, error) {
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
		matched := regexArrayMatch(tagKeyRegexes, *tag.Key)
		if matched {
			tags[*tag.Key] = *tag.Value
		}
	}
	return tags, nil
}

func compileRegexes(cfg Config) ([]*regexp.Regexp, error) {
	tagRegexes := make([]*regexp.Regexp, len(cfg.Tags))
	for i, elem := range cfg.Tags {
		regex, err := regexp.Compile(elem)
		if err != nil {
			return nil, err
		}
		tagRegexes[i] = regex
	}
	return tagRegexes, nil
}

func regexArrayMatch(arr []*regexp.Regexp, val string) bool {
	for _, elem := range arr {
		matched := elem.MatchString(val)
		if matched {
			return true
		}
	}
	return false
}
