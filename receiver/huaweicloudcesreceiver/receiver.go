package huaweicloudcesreceiver

import (
	"context"
	"math/rand"
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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

type cesReceiver struct {
	logger *zap.Logger
	client *ces.CesClient
	cancel context.CancelFunc

	host         component.Host
	nextConsumer consumer.Metrics

	config *Config
}

func newHuaweiCloudCesReceiver(settings receiver.CreateSettings, cfg *Config, next consumer.Metrics) *cesReceiver {
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
			return
		}
	}

	go rcvr.startPollingMetrics(ctx)

	return nil
}

func (rcvr *cesReceiver) createClient() (*ces.CesClient, error) {
	auth, err := basic.NewCredentialsBuilder().
		// Authentication can be configured through environment variables and other methods. Please refer to Chapter 2.4 Authentication Management
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

func (rcvr *cesReceiver) startPollingMetrics(ctx context.Context) {
	ticker := time.NewTicker(rcvr.config.CollectionInterval)

	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rcvr.logger.Info("I should start processing metrics now!")
			rcvr.nextConsumer.ConsumeMetrics(ctx, rcvr.generateMetrics(5))
			rcvr.logger.Info("gen metric")
		case <-ctx.Done():
			return
		}
	}
}

func (rcvr *cesReceiver) generateMetrics(numberOfMetrics int) pmetric.Metrics {
	metrics := pmetric.NewMetrics()

	for i := 0; i <= numberOfMetrics; i++ {

		// TODO get data from CES API and transform model
		genFakeMetrics(metrics, i)
	}

	err := rcvr.retrieveCesMetricData(metrics)
	if err != nil {
		rcvr.logger.Error(err.Error())
	}

	return metrics
}

func (rcvr *cesReceiver) retrieveCesMetricData(otlpMetrics pmetric.Metrics) error {

	request := &model.ListMetricsRequest{}
	// TODO get list of values
	// this api returns the list of metrics and their dimensions.
	// then , need to list the values for unseen period to export and send to pipeline
	response, err := rcvr.client.ListMetrics(request)
	if err != nil {
		return err
	}

	rcvr.appendToResourceMetrics(otlpMetrics, response)

	rcvr.logger.Sugar().Info(response)

	return nil
}

func (rcvr *cesReceiver) appendToResourceMetrics(metrics pmetric.Metrics, cesListMetricsResp *model.ListMetricsResponse) {
	for i := 0; i < int(cesListMetricsResp.MetaData.Count); i++ {
		resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
		resource := resourceMetrics.Resource()
		(*cesListMetricsResp.Metrics)[1]

		//rAttr := resource.Attributes()
		
	}
}

func genFakeMetrics(metrics pmetric.Metrics, i int) {
	resourceMetrics := metrics.ResourceMetrics().AppendEmpty()
	resource := resourceMetrics.Resource()
	atmAttrs := resource.Attributes()
	atmAttrs.PutInt("atm.id", int64(i))
	atmAttrs.PutStr("atm.stateid", "start")
	atmAttrs.PutStr("atm.ispnetwork", "ispnetwork")
	atmAttrs.PutStr("atm.serialnumber", "atm.SerialNumber")

	scopeMetrics := resourceMetrics.ScopeMetrics().AppendEmpty()
	atmStartTime := time.Now()
	atmDuration := 4 * time.Second
	atmTime := atmStartTime.Add(atmDuration)

	atmMetric := scopeMetrics.Metrics().AppendEmpty()
	atmMetric.SetName("atmOperationName")
	atmMetric.SetDescription("test metrics")
	gauge := atmMetric.SetEmptyGauge()
	dp := gauge.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(atmStartTime))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(atmTime))
	dp.SetDoubleValue(float64((rand.Intn(10))))
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
	rcvr.cancel()
	return nil
}
