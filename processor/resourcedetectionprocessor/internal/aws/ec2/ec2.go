// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ec2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2"

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"

	ec2provider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/aws/ec2"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/aws/ec2/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr   = "ec2"
	tagPrefix = "ec2.tag."
)

var _ internal.Detector = (*Detector)(nil)

type ec2ifaceBuilder interface {
	buildClient(ctx context.Context, region string, client *http.Client) (ec2.DescribeTagsAPIClient, error)
}

type ec2ClientBuilder struct{}

func (e *ec2ClientBuilder) buildClient(ctx context.Context, region string, client *http.Client) (ec2.DescribeTagsAPIClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithHTTPClient(client),
	)
	if err != nil {
		return nil, err
	}

	return ec2.NewFromConfig(cfg), nil
}

type Detector struct {
	metadataProvider      ec2provider.Provider
	tagKeyRegexes         []*regexp.Regexp
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	ec2ClientBuilder      ec2ifaceBuilder
	failOnMissingMetadata bool
}

func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	awsConfig, err := config.LoadDefaultConfig(context.Background())
	awsConfig.Retryer = func() aws.Retryer {
		return retry.NewStandard(func(options *retry.StandardOptions) {
			options.MaxAttempts = cfg.MaxAttempts
			options.MaxBackoff = cfg.MaxBackoff
		})
	}
	if err != nil {
		return nil, err
	}
	tagKeyRegexes, err := compileRegexes(cfg)
	if err != nil {
		return nil, err
	}

	return &Detector{
		metadataProvider:      ec2provider.NewProvider(awsConfig),
		tagKeyRegexes:         tagKeyRegexes,
		logger:                set.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		ec2ClientBuilder:      &ec2ClientBuilder{},
		failOnMissingMetadata: cfg.FailOnMissingMetadata,
	}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if _, err = d.metadataProvider.InstanceID(ctx); err != nil {
		d.logger.Debug("EC2 metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	meta, err := d.metadataProvider.Get(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting identity document: %w", err)
	}

	hostname, err := d.metadataProvider.Hostname(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting hostname: %w", err)
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAWS.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformAWSEC2.Value.AsString())
	d.rb.SetCloudRegion(meta.Region)
	d.rb.SetCloudAccountID(meta.AccountID)
	d.rb.SetCloudAvailabilityZone(meta.AvailabilityZone)
	d.rb.SetHostID(meta.InstanceID)
	d.rb.SetHostImageID(meta.ImageID)
	d.rb.SetHostType(meta.InstanceType)
	d.rb.SetHostName(hostname)
	res := d.rb.Emit()

	if len(d.tagKeyRegexes) != 0 {
		httpClient := getClientConfig(ctx, d.logger)
		ec2Client, err := d.ec2ClientBuilder.buildClient(ctx, meta.Region, httpClient)
		if err != nil {
			d.logger.Warn("failed to build ec2 client", zap.Error(err))
			return res, conventions.SchemaURL, nil
		}
		tags, err := fetchEC2Tags(ctx, ec2Client, meta.InstanceID, d.tagKeyRegexes)
		if err != nil {
			d.logger.Warn("failed fetching ec2 instance tags", zap.Error(err))
		} else {
			for key, val := range tags {
				res.Attributes().PutStr(tagPrefix+key, val)
			}
		}
	}
	return res, conventions.SchemaURL, nil
}

func getClientConfig(ctx context.Context, logger *zap.Logger) *http.Client {
	client, err := internal.ClientFromContext(ctx)
	if err != nil {
		client = http.DefaultClient
		logger.Debug("Error retrieving client from context thus creating default", zap.Error(err))
	}
	return client
}

func fetchEC2Tags(ctx context.Context, svc ec2.DescribeTagsAPIClient, instanceID string, tagKeyRegexes []*regexp.Regexp) (map[string]string, error) {
	ec2Tags, err := svc.DescribeTags(ctx, &ec2.DescribeTagsInput{
		Filters: []types.Filter{{
			Name:   aws.String("resource-id"),
			Values: []string{instanceID},
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
