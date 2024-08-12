package huaweicloudcesreceiver

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/config"
	ces "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/model"
	"github.com/huaweicloud/huaweicloud-sdk-go-v3/services/ces/v1/region"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver/internal"
)

type cesReceiver struct {
	logger *zap.Logger
	client *ces.CesClient
	cancel context.CancelFunc

	host             component.Host
	nextConsumer     consumer.Metrics
	lastUsedFinishTs time.Time
	config           *Config
}

func newHuaweiCloudCesReceiver(settings receiver.Settings, cfg *Config, next consumer.Metrics) *cesReceiver {
	rcvr := &cesReceiver{
		logger:       settings.Logger,
		config:       cfg,
		nextConsumer: next,
	}
	return rcvr
}

func (rcvr *cesReceiver) Start(ctx context.Context, host component.Host) (err error) {
	rcvr.host = host
	ctx, rcvr.cancel = context.WithCancel(ctx)

	if rcvr.client == nil {
		rcvr.client, err = rcvr.createClient()
		if err != nil {
			rcvr.logger.Error(err.Error())
			return
		}
	}

	go func() {
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
	}()
	return nil
}

func (rcvr *cesReceiver) createClient() (*ces.CesClient, error) {
	auth, err := basic.NewCredentialsBuilder().
		// Authentication can be configured through environment variables and other methods.
		// Please refer to Chapter 2.4 Authentication Management
		WithAk(os.Getenv("HUAWEICLOUD_SDK_AK")).
		WithSk(os.Getenv("HUAWEICLOUD_SDK_SK")).
		WithProjectId(rcvr.config.ProjectId).
		SafeBuild()

	if err != nil {
		return nil, err
	}

	httpConfig := config.DefaultHttpConfig().
		WithIgnoreSSLVerification(rcvr.config.NoVerifySSL)

	if rcvr.config.ProxyAddress != "" {
		proxy, err := rcvr.configureHttpProxy()
		if err != nil {
			return nil, err
		}
		httpConfig = config.DefaultHttpConfig().WithProxy(proxy)
	}

	r, err := region.SafeValueOf(rcvr.config.RegionName)
	if err != nil {
		return nil, err
	}

	hcHttpConfig, err := ces.CesClientBuilder().
		WithRegion(r).
		WithCredential(auth).
		WithHttpConfig(httpConfig).
		SafeBuild()

	if err != nil {
		return nil, err
	}

	client := ces.NewCesClient(hcHttpConfig)

	return client, nil
}

func (rcvr *cesReceiver) pollMetricsAndConsume(ctx context.Context) error {
	metricDefinitions, err := rcvr.listMetricDefinitions()
	if err != nil {
		return err
	}
	cesMetrics, err := rcvr.listDataPoints(metricDefinitions)
	if err != nil {
		rcvr.logger.Error(err.Error())
		return err
	}
	metrics := internal.ConvertCESMetricsToOTLP(rcvr.config.ProjectId, rcvr.config.RegionName, rcvr.config.Filter, cesMetrics)
	if err := rcvr.nextConsumer.ConsumeMetrics(ctx, metrics); err != nil {
		return err
	}
	return nil
}

func (rcvr *cesReceiver) listMetricDefinitions() ([]model.MetricInfoList, error) {
	response, err := rcvr.client.ListMetrics(&model.ListMetricsRequest{})
	if err != nil {
		return []model.MetricInfoList{}, err
	}
	if response.Metrics == nil || len((*response.Metrics)) == 0 {
		return []model.MetricInfoList{}, errors.New("empty list of metric definitions")
	}

	return *response.Metrics, nil
}

func convertMetricInfoListArrayToMetricInfoArray(infoListArray []model.MetricInfoList) []model.MetricInfo {
	infoArray := make([]model.MetricInfo, len(infoListArray))
	for i, infoList := range infoListArray {
		infoArray[i] = model.MetricInfo{
			Namespace:  infoList.Namespace,
			MetricName: infoList.MetricName,
			Dimensions: infoList.Dimensions,
		}
	}
	return infoArray
}

func (rcvr *cesReceiver) listDataPoints(metricDefinitions []model.MetricInfoList) ([]model.BatchMetricData, error) {
	// TODO: Handle delayed metrics. CES accepts metrics with up to a 30-minute delay.
	// If the request is based on the current time ('now'), it may miss metrics delayed by a minute or more,
	// as the next request would exclude them. Consider adding a delay configuration to account for this.
	// TODO: Implement deduplication: There may be a need for deduplication, possibly using a Processor to ensure unique metrics are processed.
	to := time.Now()
	var from time.Time
	if rcvr.lastUsedFinishTs.IsZero() {
		from = to.Add(-1 * rcvr.config.CollectionInterval)
	} else {
		from = rcvr.lastUsedFinishTs
	}
	rcvr.lastUsedFinishTs = to
	metrics := convertMetricInfoListArrayToMetricInfoArray(metricDefinitions)

	response, err := rcvr.client.BatchListMetricData(&model.BatchListMetricDataRequest{
		Body: &model.BatchListMetricDataRequestBody{
			Metrics: metrics,
			Period:  strconv.Itoa(rcvr.config.Period),
			Filter:  rcvr.config.Filter,
			From:    from.UnixMilli(),
			To:      to.UnixMilli(),
		},
	})
	if err != nil {
		return []model.BatchMetricData{}, err
	}
	if response.Metrics == nil || len(*response.Metrics) == 0 {
		return []model.BatchMetricData{}, errors.New("empty list of metric data")
	}
	return *response.Metrics, nil
}

func (rcvr *cesReceiver) configureHttpProxy() (*config.Proxy, error) {
	proxyUrl, err := url.Parse(rcvr.config.ProxyAddress)
	if err != nil {
		return nil, err
	}

	proxy := config.NewProxy().
		WithSchema(proxyUrl.Scheme).
		WithHost(proxyUrl.Hostname())
	if len(proxyUrl.Port()) > 0 {
		if i, err := strconv.Atoi(proxyUrl.Port()); err == nil {
			proxy = proxy.WithPort(i)
		}
	}

	// Configure the username and password if the proxy requires authentication
	if len(rcvr.config.ProxyUser) > 0 {
		proxy = proxy.
			WithUsername(rcvr.config.ProxyUser).
			WithPassword(rcvr.config.ProxyPassword)
	}
	return proxy, nil
}

func (rcvr *cesReceiver) Shutdown(ctx context.Context) error {
	if rcvr.cancel != nil {
		rcvr.cancel()
	}
	return nil
}
