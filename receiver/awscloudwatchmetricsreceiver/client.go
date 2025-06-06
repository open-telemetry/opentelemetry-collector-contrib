// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchmetricsreceiver

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"go.uber.org/zap"
)

type client interface {
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

type cloudWatchClient struct {
	client *cloudwatch.Client
	logger *zap.Logger
}

func newCloudWatchClient(cfg *Config, logger *zap.Logger) (*cloudWatchClient, error) {
	cfgOptions := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.IMDSEndpoint != "" {
		cfgOptions = append(cfgOptions, config.WithEC2IMDSEndpoint(cfg.IMDSEndpoint))
	}

	if cfg.Profile != "" {
		cfgOptions = append(cfgOptions, config.WithSharedConfigProfile(cfg.Profile))
	}

	awsCfg, err := config.LoadDefaultConfig(context.Background(), cfgOptions...)
	if err != nil {
		return nil, err
	}

	return &cloudWatchClient{
		client: cloudwatch.NewFromConfig(awsCfg),
		logger: logger,
	}, nil
}

func (c *cloudWatchClient) GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error) {
	c.logger.Debug("Fetching metric data from CloudWatch",
		zap.Time("startTime", *params.StartTime),
		zap.Time("endTime", *params.EndTime),
		zap.Int("metricDataQueries", len(params.MetricDataQueries)))

	return c.client.GetMetricData(ctx, params, optFns...)
}

// buildMetricDataQuery constructs a MetricDataQuery for the given metric configuration
func buildMetricDataQuery(namespace, metricName string, period time.Duration, stat string, dimensions []MetricDimensionsConfig) types.MetricDataQuery {
	dimensionList := make([]types.Dimension, len(dimensions))
	for i, d := range dimensions {
		dimensionList[i] = types.Dimension{
			Name:  aws.String(d.Name),
			Value: aws.String(d.Value),
		}
	}

	metric := types.Metric{
		Namespace:  aws.String(namespace),
		MetricName: aws.String(metricName),
		Dimensions: dimensionList,
	}

	return types.MetricDataQuery{
		Id:         aws.String(metricName), // Using metric name as ID for simplicity
		MetricStat: &types.MetricStat{
			Metric: &metric,
			Period: aws.Int32(int32(period.Seconds())),
			Stat:   aws.String(stat),
		},
	}
} 