// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package datadogexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/config"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes"
	"github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes/source"
	otlpmetrics "github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics"
	"github.com/DataDog/sketches-go/ddsketch"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	zorkian "gopkg.in/zorkian/go-datadog-api.v2"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/internal/metrics/sketches"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/clientutil"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/hostmetadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/scrub"
	pkgdatadog "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog"
	datadogconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"

	"crypto/tls"

	pb "github.com/DataDog/datadog-agent/pkg/proto/pbgo/trace"
)

type metricsExporter struct {
	params           exporter.Settings
	cfg              *datadogconfig.Config
	agntConfig       *config.AgentConfig
	ctx              context.Context
	client           *zorkian.Client
	metricsAPI       *datadogV2.MetricsApi
	tr               *otlpmetrics.Translator
	scrubber         scrub.Scrubber
	retrier          *clientutil.Retrier
	onceMetadata     *sync.Once
	sourceProvider   source.Provider
	metadataReporter *inframetadata.Reporter
	// getPushTime returns a Unix time in nanoseconds, representing the time pushing metrics.
	// It will be overwritten in tests.
	getPushTime func() uint64

	gatewayUsage *attributes.GatewayUsage
	statsOut     chan []byte
}

func newMetricsExporter(
	ctx context.Context,
	params exporter.Settings,
	cfg *datadogconfig.Config,
	agntConfig *config.AgentConfig,
	onceMetadata *sync.Once,
	attrsTranslator *attributes.Translator,
	sourceProvider source.Provider,
	metadataReporter *inframetadata.Reporter,
	statsOut chan []byte,
	gatewayUsage *attributes.GatewayUsage,
) (*metricsExporter, error) {
	options := cfg.Metrics.ToTranslatorOpts()
	options = append(options, otlpmetrics.WithFallbackSourceProvider(sourceProvider))
	options = append(options, otlpmetrics.WithStatsOut(statsOut))
	if pkgdatadog.MetricRemappingDisabledFeatureGate.IsEnabled() {
		params.Logger.Warn("Metric remapping is disabled in the Datadog exporter. OpenTelemetry metrics must be mapped to Datadog semantics before metrics are exported to Datadog (ex: via a processor).")
	} else {
		options = append(options, otlpmetrics.WithRemapping())
	}

	tr, err := otlpmetrics.NewTranslator(params.TelemetrySettings, attrsTranslator, options...)
	if err != nil {
		return nil, err
	}

	scrubber := scrub.NewScrubber()
	exporter := &metricsExporter{
		params:           params,
		cfg:              cfg,
		ctx:              ctx,
		agntConfig:       agntConfig,
		tr:               tr,
		scrubber:         scrubber,
		retrier:          clientutil.NewRetrier(params.Logger, cfg.BackOffConfig, scrubber),
		onceMetadata:     onceMetadata,
		sourceProvider:   sourceProvider,
		getPushTime:      func() uint64 { return uint64(time.Now().UTC().UnixNano()) },
		metadataReporter: metadataReporter,
		gatewayUsage:     gatewayUsage,
		statsOut:         statsOut,
	}
	errchan := make(chan error)
	if isMetricExportV2Enabled() {
		apiClient := clientutil.CreateAPIClient(
			params.BuildInfo,
			cfg.Metrics.Endpoint,
			cfg.ClientConfig)
		go func() { errchan <- clientutil.ValidateAPIKey(ctx, string(cfg.API.Key), params.Logger, apiClient) }()
		exporter.metricsAPI = datadogV2.NewMetricsApi(apiClient)
	} else {
		client := clientutil.CreateZorkianClient(string(cfg.API.Key), cfg.Metrics.Endpoint)
		client.ExtraHeader["User-Agent"] = clientutil.UserAgent(params.BuildInfo)
		client.HttpClient = clientutil.NewHTTPClient(cfg.ClientConfig)
		go func() { errchan <- clientutil.ValidateAPIKeyZorkian(params.Logger, client) }()
		exporter.client = client
	}
	if cfg.API.FailOnInvalidKey {
		err = <-errchan
		if err != nil {
			return nil, err
		}
	}
	return exporter, nil
}

func (exp *metricsExporter) pushSketches(ctx context.Context, sl sketches.SketchSeriesList) error {
	payload, err := sl.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal sketches: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx,
		http.MethodPost,
		exp.cfg.Metrics.Endpoint+sketches.SketchSeriesEndpoint,
		bytes.NewBuffer(payload),
	)
	if err != nil {
		return fmt.Errorf("failed to build sketches HTTP request: %w", err)
	}

	clientutil.SetDDHeaders(req.Header, exp.params.BuildInfo, string(exp.cfg.API.Key))
	clientutil.SetExtraHeaders(req.Header, clientutil.ProtobufHeaders)
	var resp *http.Response
	if isMetricExportV2Enabled() {
		resp, err = exp.metricsAPI.Client.Cfg.HTTPClient.Do(req)
	} else {
		// Create an HTTP client with TLS verification disabled (insecure)
		insecureTransport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client := &http.Client{Transport: insecureTransport}
		resp, err = client.Do(req)
	}

	if err != nil {
		return clientutil.WrapError(fmt.Errorf("failed to do sketches HTTP request: %w", err), resp)
	}
	defer resp.Body.Close()

	// We must read the full response body from the http request to ensure that connections can be
	// properly re-used. https://pkg.go.dev/net/http#Client.Do
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return clientutil.WrapError(fmt.Errorf("failed to read response body from sketches HTTP request: %w", err), resp)
	}

	if resp.StatusCode >= 400 {
		return clientutil.WrapError(fmt.Errorf("error when sending payload to %s: %s", sketches.SketchSeriesEndpoint, resp.Status), resp)
	}
	return nil
}

func (exp *metricsExporter) PushMetricsDataScrubbed(ctx context.Context, md pmetric.Metrics) error {
	return exp.scrubber.Scrub(exp.PushMetricsData(ctx, md))
}

func (exp *metricsExporter) PushMetricsData(ctx context.Context, md pmetric.Metrics) error {
	if exp.cfg.HostMetadata.Enabled {
		// Start host metadata with resource attributes from
		// the first payload.
		exp.onceMetadata.Do(func() {
			attrs := pcommon.NewMap()
			if md.ResourceMetrics().Len() > 0 {
				attrs = md.ResourceMetrics().At(0).Resource().Attributes()
			}
			go hostmetadata.RunPusher(exp.ctx, exp.params, newMetadataConfigfromConfig(exp.cfg), exp.sourceProvider, attrs, exp.metadataReporter)
		})

		// Consume resources for host metadata
		for i := 0; i < md.ResourceMetrics().Len(); i++ {
			res := md.ResourceMetrics().At(i).Resource()
			consumeResource(exp.metadataReporter, res, exp.params.Logger)
		}
	}
	var consumer otlpmetrics.Consumer
	if isMetricExportV2Enabled() {
		consumer = metrics.NewConsumer(exp.gatewayUsage)
	} else {
		consumer = metrics.NewZorkianConsumer()
	}

	processAndReturnErrorIf := func(condition bool) error {
		if !condition {
			return nil
		}
		return errors.New("cannot unmarshal OTLP stats")
	}
	fmt.Println("********** PUSH STATS **********")

	rms := md.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)

		// Get values from resource attributes
		hostname := ""
		env := ""
		service := ""
		version := ""
		hostnameVal, ok := rm.Resource().Attributes().Get("host.name")
		if ok {
			hostname = hostnameVal.AsString()
		}
		envVal, ok := rm.Resource().Attributes().Get("dd.env")
		if ok {
			env = envVal.AsString()
		}
		serviceVal, ok := rm.Resource().Attributes().Get("dd.service")
		if ok {
			service = serviceVal.AsString()
		}
		if service == "" {
			serviceVal, ok = rm.Resource().Attributes().Get("service.name")
			if ok {
				service = serviceVal.AsString()
			}
		}
		versionVal, ok := rm.Resource().Attributes().Get("dd.version")
		if ok {
			version = versionVal.AsString()
		}

		// Defaults for missing values
		if hostname == "" {
			hostname = "fallbackHostname"
		}
		if env == "" {
			env = "default"
		}
		if service == "" {
			service = "otlpresourcenoservicename"
		}
		// - No default for version

		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			sm := sms.At(j)
			scopeName := sm.Scope().Name()
			if scopeName != "datadog.trace.metrics" {
				continue
			}

			sps := sm.Metrics()
			for k := 0; k < sps.Len(); k++ {
				sp := sps.At(k)
				if err := processAndReturnErrorIf(sp.Type() != pmetric.MetricTypeHistogram); err != nil {
					return err
				}
				hist := sp.Histogram()
				for l := 0; l < hist.DataPoints().Len(); l++ {
					dp := hist.DataPoints().At(l)

					duration := dp.Sum()

					clientStatsBucket := pb.ClientStatsBucket{
						Start:    uint64(dp.Timestamp().AsTime().UnixNano()),
						Duration: uint64(duration),
						Stats:    make([]*pb.ClientGroupedStats, 0),
					}

					// Reconstructing the key
					name := ""
					nameVal, ok := dp.Attributes().Get("Name")
					if ok {
						name = nameVal.AsString()
					}

					resource := ""
					resourceVal, ok := dp.Attributes().Get("Resource")
					if ok {
						resource = resourceVal.AsString()
					}

					typ := ""
					typeVal, ok := dp.Attributes().Get("Type")
					if ok {
						typ = typeVal.AsString()
					}

					statusCode := uint32(0)
					statusCodeVal, ok := dp.Attributes().Get("Status")
					if ok {
						statusCode = uint32(statusCodeVal.Int())
					}
					if statusCode == 0 {
						statusCode = 200
					}

					isTraceRoot := pb.Trilean_NOT_SET
					topLevelVal, ok := dp.Attributes().Get("TopLevel")
					if ok {
						if topLevelVal.Bool() {
							isTraceRoot = pb.Trilean_TRUE
						} else {
							isTraceRoot = pb.Trilean_FALSE
						}
					}

					isErrorDistribtion := false
					errorVal, ok := dp.Attributes().Get("Error")
					if ok {
						isErrorDistribtion = errorVal.Bool()
					}

					// Reconstructing the value
					clientGroupedStatsValue := pb.ClientGroupedStats{
						Service:        service,
						Name:           name,
						Resource:       resource,
						Type:           typ,
						HTTPStatusCode: statusCode,
						IsTraceRoot:    isTraceRoot,
						Duration:       uint64(duration),
					}
					count := dp.Count()
					if isTraceRoot == pb.Trilean_TRUE {
						clientGroupedStatsValue.TopLevelHits = count
					}

					// XXX chosen to match the trace metrics computation in datadogconnector
					sketch, err := ddsketch.LogCollapsingLowestDenseDDSketch(0.01, 2048)
					bounds := dp.ExplicitBounds()
					buckets := dp.BucketCounts()
					for i := 1; i < buckets.Len(); i++ {
						bucketCount := buckets.At(i)
						boundaryValue := bounds.At(i - 1)
						sketch.AddWithCount(boundaryValue, float64(bucketCount))
					}
					if err := processAndReturnErrorIf(err != nil); err != nil {
						return err
					}
					sketchProto := sketch.ToProto()
					sketchBytes, err := proto.Marshal(sketchProto)
					if err != nil {
						return err
					}
					if isErrorDistribtion {
						clientGroupedStatsValue.Errors = count
						clientGroupedStatsValue.ErrorSummary = sketchBytes
					} else {
						clientGroupedStatsValue.Hits = count
						clientGroupedStatsValue.OkSummary = sketchBytes
					}

					clientStatsBucket.Stats = append(clientStatsBucket.Stats, &clientGroupedStatsValue)
					clientStatsPayload := pb.ClientStatsPayload{
						Env:      env,
						Service:  service,
						Hostname: hostname,
						Version:  version,
						Stats: []*pb.ClientStatsBucket{
							&clientStatsBucket,
						},
					}
					payload := &pb.StatsPayload{
						Stats:          []*pb.ClientStatsPayload{&clientStatsPayload},
						ClientComputed: true,
						SplitPayload:   true,
						AgentHostname:  exp.agntConfig.Hostname,
						AgentEnv:       exp.agntConfig.DefaultEnv,
						AgentVersion:   exp.agntConfig.AgentVersion,
					}

					apiKey := os.Getenv("DD_API_KEY")
					ddSite := os.Getenv("DD_SITE")
					if ddSite == "" {
						ddSite = "datadoghq.com"
					}
					url := fmt.Sprintf("https://trace.agent.%s/api/v0.2/stats", ddSite)

					w := bytes.NewBuffer(nil)
					gz, err := gzip.NewWriterLevel(w, gzip.BestSpeed)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to create gzip writer: %v\n", err)
						os.Exit(1)
					}
					msgp.Encode(gz, payload)
					gz.Close()

					req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, w)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to create HTTP request: %v\n", err)
						os.Exit(1)
					}
					req.Header.Set("Dd-Api-Key", apiKey)
					req.Header.Set("X-Datadog-Reported-Languages", "go")
					req.Header.Set("Content-Type", "application/msgpack")
					req.Header.Set("Content-Encoding", "gzip")

					client := &http.Client{}
					resp, err := client.Do(req)
					if err != nil {
						fmt.Fprintf(os.Stderr, "failed to execute HTTP request: %v\n", err)
						os.Exit(1)
					}
					defer resp.Body.Close()

					// Discard the body so the connection can be reused
					if resp.StatusCode/100 != 2 {
						fmt.Fprintf(os.Stderr, "status code %d returned\n", resp.StatusCode)
						fmt.Fprintf(os.Stderr, "body: %s\n", resp.Body)
					} else {
						fmt.Println("Payload sent successfully!")
						fmt.Println(">> resp.StatusCode", resp.StatusCode)
					}

					io.Copy(io.Discard, resp.Body)
				}
			}
		}
	}

	metadata, err := exp.tr.MapMetrics(ctx, md, consumer, exp.gatewayUsage)
	if err != nil {
		return fmt.Errorf("failed to map metrics: %w", err)
	}
	src, err := exp.sourceProvider.Source(ctx)
	if err != nil {
		return err
	}
	var tags []string
	if src.Kind == source.AWSECSFargateKind {
		tags = append(tags, exp.cfg.HostMetadata.Tags...)
	}

	var sl sketches.SketchSeriesList
	var errs []error
	if isMetricExportV2Enabled() {
		var ms []datadogV2.MetricSeries
		ms, sl = consumer.(*metrics.Consumer).All(exp.getPushTime(), exp.params.BuildInfo, tags, metadata)
		if len(ms) > 0 {
			exp.params.Logger.Debug("exporting native Datadog payload", zap.Any("metric", ms))
			_, experr := exp.retrier.DoWithRetries(ctx, func(context.Context) error {
				ctx = clientutil.GetRequestContext(ctx, string(exp.cfg.API.Key))
				_, httpresp, merr := exp.metricsAPI.SubmitMetrics(ctx, datadogV2.MetricPayload{Series: ms}, *clientutil.GZipSubmitMetricsOptionalParameters)
				return clientutil.WrapError(merr, httpresp)
			})
			errs = append(errs, experr)
		}
	} else {
		var ms []zorkian.Metric
		ms, sl = consumer.(*metrics.ZorkianConsumer).All(exp.getPushTime(), exp.params.BuildInfo, tags)
		if len(ms) > 0 {
			exp.params.Logger.Debug("exporting Zorkian Datadog payload", zap.Any("metric", ms))
			_, experr := exp.retrier.DoWithRetries(ctx, func(context.Context) error {
				return exp.client.PostMetrics(ms)
			})
			errs = append(errs, experr)
		}
	}

	if len(sl) > 0 {
		exp.params.Logger.Debug("exporting sketches payload", zap.Any("sketches", sl))
		_, experr := exp.retrier.DoWithRetries(ctx, func(ctx context.Context) error {
			return exp.pushSketches(ctx, sl)
		})
		errs = append(errs, experr)
	}

	return errors.Join(errs...)
}
