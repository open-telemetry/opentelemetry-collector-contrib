// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver"

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	ces "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/region"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	internal "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal"
)

const (
	// See https://support.huaweicloud.com/intl/en-us/devg-apisign/api-sign-errorcode.html
	requestThrottledErrMsg = "APIGW.0308"
)

type cesReceiver struct {
	logger *zap.Logger
	client internal.CesClient
	cancel context.CancelFunc

	host         component.Host
	nextConsumer consumer.Metrics
	lastSeenTs   map[string]time.Time
	config       *Config
	shutdownChan chan struct{}
}

func newHuaweiCloudCesReceiver(settings receiver.Settings, cfg *Config, next consumer.Metrics) *cesReceiver {
	rcvr := &cesReceiver{
		logger:       settings.Logger,
		config:       cfg,
		nextConsumer: next,
		lastSeenTs:   make(map[string]time.Time),
		shutdownChan: make(chan struct{}, 1),
	}
	return rcvr
}

func (rcvr *cesReceiver) Start(ctx context.Context, host component.Host) error {
	rcvr.host = host
	ctx, rcvr.cancel = context.WithCancel(ctx)

	if rcvr.client == nil {
		client, err := rcvr.createClient()
		if err != nil {
			rcvr.logger.Error(err.Error())
			return nil
		}
		rcvr.client = client
	}

	go rcvr.startReadingMetrics(ctx)
	return nil
}

func (rcvr *cesReceiver) startReadingMetrics(ctx context.Context) {
	if rcvr.config.InitialDelay > 0 {
		<-time.After(rcvr.config.InitialDelay)
	}
	if err := rcvr.pollMetricsAndConsume(ctx); err != nil {
		rcvr.logger.Error(err.Error())
	}
	ticker := time.NewTicker(rcvr.config.CollectionInterval)

	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			//  TODO: Improve error handling for client-server interactions
			//  The current implementation lacks robust error handling, especially for
			//  scenarios such as service unavailability, timeouts, and request errors.
			//  - Investigate how to handle service unavailability or timeouts gracefully.
			//  - Implement appropriate actions or retries for different types of request errors.
			//  - Refer to the Huawei SDK documentation to identify
			//    all possible client/request errors and determine how to manage them.
			//  - Consider implementing custom error messages or fallback mechanisms for critical failures.

			if err := rcvr.pollMetricsAndConsume(ctx); err != nil {
				rcvr.logger.Error(err.Error())
			}
		case <-ctx.Done():
			return
		}
	}
}

func (rcvr *cesReceiver) createClient() (*ces.CesClient, error) {
	auth, err := basic.NewCredentialsBuilder().
		WithAk(string(rcvr.config.AccessKey)).
		WithSk(string(rcvr.config.SecretKey)).
		WithProjectId(rcvr.config.ProjectID).
		SafeBuild()
	if err != nil {
		return nil, err
	}

	httpConfig, err := createHTTPConfig(rcvr.config.huaweiSessionConfig)
	if err != nil {
		return nil, err
	}
	r, err := region.SafeValueOf(rcvr.config.RegionID)
	if err != nil {
		return nil, err
	}

	hcHTTPConfig, err := ces.CesClientBuilder().
		WithRegion(r).
		WithCredential(auth).
		WithHttpConfig(httpConfig).
		SafeBuild()
	if err != nil {
		return nil, err
	}

	client := ces.NewCesClient(hcHTTPConfig)

	return client, nil
}

func (rcvr *cesReceiver) pollMetricsAndConsume(ctx context.Context) error {
	if rcvr.client == nil {
		return errors.New("invalid client")
	}
	metricDefinitions, err := rcvr.listMetricDefinitions(ctx)
	if err != nil {
		return err
	}
	metrics := rcvr.listDataPoints(ctx, metricDefinitions)
	otpMetrics := internal.ConvertCESMetricsToOTLP(rcvr.config.ProjectID, rcvr.config.RegionID, rcvr.config.Filter, metrics)
	if err := rcvr.nextConsumer.ConsumeMetrics(ctx, otpMetrics); err != nil {
		return err
	}
	return nil
}

func (rcvr *cesReceiver) listMetricDefinitions(ctx context.Context) ([]model.MetricInfoList, error) {
	response, err := internal.MakeAPICallWithRetry(
		ctx,
		rcvr.shutdownChan,
		rcvr.logger,
		func() (*model.ListMetricsResponse, error) {
			return rcvr.client.ListMetrics(&model.ListMetricsRequest{})
		},
		func(err error) bool { return strings.Contains(err.Error(), requestThrottledErrMsg) },
		internal.NewExponentialBackOff(&rcvr.config.BackOffConfig),
	)
	if err != nil {
		return []model.MetricInfoList{}, err
	}
	if response == nil || response.Metrics == nil || len((*response.Metrics)) == 0 {
		return []model.MetricInfoList{}, errors.New("unexpected empty list of metric definitions")
	}

	return *response.Metrics, nil
}

// listDataPoints retrieves data points for a list of metric definitions.
// The function performs the following operations:
//  1. Generates a unique key for each metric definition (at least one dimenstion is required) and checks for duplicates.
//  2. Determines the time range (from-to) for fetching the metric data points, using the current timestamp
//     and the last-seen timestamp for each metric.
//  3. Fetches data points for each metric definition.
//  4. Updates the last-seen timestamp for each metric based on the most recent data point timestamp.
//  5. Returns a map of metric keys to their corresponding MetricData, containing all fetched data points.
//
// Parameters:
//   - ctx: Context for controlling cancellation and deadlines.
//   - metricDefinitions: A slice of MetricInfoList containing the definitions of metrics to be fetched.
//
// Returns:
//   - A map where each key is a unique metric identifier and each value is the associated MetricData.
func (rcvr *cesReceiver) listDataPoints(ctx context.Context, metricDefinitions []model.MetricInfoList) map[string][]*internal.MetricData {
	// TODO: Implement deduplication: There may be a need for deduplication, possibly using a Processor to ensure unique metrics are processed.
	to := time.Now()
	metrics := make(map[string][]*internal.MetricData)
	for _, metricDefinition := range metricDefinitions {
		if len(metricDefinition.Dimensions) == 0 {
			rcvr.logger.Warn("metric has 0 dimensions. skipping it", zap.String("metricName", metricDefinition.MetricName))
			continue
		}
		key := internal.GetMetricKey(metricDefinition)
		from, ok := rcvr.lastSeenTs[key]
		if !ok {
			from = to.Add(-1 * rcvr.config.CollectionInterval)
		}
		resp, dpErr := rcvr.listDataPointsForMetric(ctx, from, to, metricDefinition)
		if dpErr != nil {
			rcvr.logger.Warn(fmt.Sprintf("unable to get datapoints for metric name %+v", metricDefinition), zap.Error(dpErr))
		}
		var datapoints []model.Datapoint
		if resp != nil && resp.Datapoints != nil {
			datapoints = *resp.Datapoints

			var maxdpTs int64
			for _, dp := range datapoints {
				if dp.Timestamp > maxdpTs {
					maxdpTs = dp.Timestamp
				}
			}
			if maxdpTs > rcvr.lastSeenTs[key].UnixMilli() {
				rcvr.lastSeenTs[key] = time.UnixMilli(maxdpTs)
			}
		}
		metrics[metricDefinition.Namespace] = append(metrics[metricDefinition.Namespace], &internal.MetricData{
			MetricName: metricDefinition.MetricName,
			Dimensions: metricDefinition.Dimensions,
			Namespace:  metricDefinition.Namespace,
			Unit:       metricDefinition.Unit,
			Datapoints: datapoints,
		})
	}
	return metrics
}

func (rcvr *cesReceiver) listDataPointsForMetric(ctx context.Context, from, to time.Time, infoList model.MetricInfoList) (*model.ShowMetricDataResponse, error) {
	return internal.MakeAPICallWithRetry(
		ctx,
		rcvr.shutdownChan,
		rcvr.logger,
		func() (*model.ShowMetricDataResponse, error) {
			return rcvr.client.ShowMetricData(&model.ShowMetricDataRequest{
				Namespace:  infoList.Namespace,
				MetricName: infoList.MetricName,
				Dim0:       infoList.Dimensions[0].Name + "," + infoList.Dimensions[0].Value,
				Dim1:       internal.GetDimension(infoList.Dimensions, 1),
				Dim2:       internal.GetDimension(infoList.Dimensions, 2),
				Dim3:       internal.GetDimension(infoList.Dimensions, 3),
				Period:     rcvr.config.Period,
				Filter:     validFilters[rcvr.config.Filter],
				From:       from.UnixMilli(),
				To:         to.UnixMilli(),
			})
		},
		func(err error) bool { return strings.Contains(err.Error(), requestThrottledErrMsg) },
		internal.NewExponentialBackOff(&rcvr.config.BackOffConfig),
	)
}

func (rcvr *cesReceiver) Shutdown(_ context.Context) error {
	if rcvr.cancel != nil {
		rcvr.cancel()
	}
	rcvr.shutdownChan <- struct{}{}
	close(rcvr.shutdownChan)
	return nil
}
