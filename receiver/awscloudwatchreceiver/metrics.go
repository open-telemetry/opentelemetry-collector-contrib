// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awscloudwatchreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/receiver"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver/internal/metadata"
)

const (
	// maxGetMetricDataQueries is the AWS limit per GetMetricData request.
	maxGetMetricDataQueries = 500

	// statsPerMetric is the number of CloudWatch statistics we request per metric (Sum, SampleCount, Minimum, Maximum).
	statsPerMetric = 4

	// stat index constants within a metric's sub-queries
	statIdxSum   = 0
	statIdxCount = 1
	statIdxMin   = 2
	statIdxMax   = 3
)

type cloudWatchMetricsScraper struct {
	settings           receiver.Settings
	cfg                *Config
	period             time.Duration
	delay              time.Duration
	collectionInterval time.Duration
	metrics            []MetricQuery
	discovery          *MetricsDiscoveryConfig
	client             metricsClient
	stsClient          stsClient
	// accountID is the resolved AWS account ID reported as cloud.account.id.
	// It is empty until resolution succeeds.
	accountID string
	// accountIDResolved is set once the account ID has been resolved successfully. Until
	// then resolution is retried on each scrape, so a transient STS failure on the first
	// attempt does not permanently leave cloud.account.id unset. Resolution runs lazily on
	// scrape (not in start()) so that start() performs no network calls.
	accountIDResolved bool
}

type metricsClient interface {
	ListMetrics(ctx context.Context, params *cloudwatch.ListMetricsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.ListMetricsOutput, error)
	GetMetricData(ctx context.Context, params *cloudwatch.GetMetricDataInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricDataOutput, error)
}

// stsClient resolves the AWS account ID of the active credentials via GetCallerIdentity.
type stsClient interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func newCloudWatchMetricsScraper(cfg *Config, settings receiver.Settings) *cloudWatchMetricsScraper {
	var discovery *MetricsDiscoveryConfig
	if d := cfg.Metrics.Discovery; d != nil {
		discoveryCfg := *d
		if discoveryCfg.Limit <= 0 {
			discoveryCfg.Limit = defaultMetricsDiscoverLimit
		}
		discovery = &discoveryCfg
	}
	return &cloudWatchMetricsScraper{
		settings:           settings,
		cfg:                cfg,
		period:             cfg.Metrics.Period,
		delay:              cfg.Metrics.Delay,
		collectionInterval: cfg.Metrics.CollectionInterval,
		metrics:            cfg.Metrics.Queries,
		discovery:          discovery,
	}
}

func (s *cloudWatchMetricsScraper) start(ctx context.Context, _ component.Host) error {
	s.settings.Logger.Debug("initializing CloudWatch client", zap.String("region", s.cfg.Region))
	opts := []func(*config.LoadOptions) error{config.WithRegion(s.cfg.Region)}
	if s.cfg.IMDSEndpoint != "" {
		opts = append(opts, config.WithEC2IMDSEndpoint(s.cfg.IMDSEndpoint))
	}
	if s.cfg.Profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(s.cfg.Profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return err
	}
	s.client = cloudwatch.NewFromConfig(cfg)
	// Build the STS client from the same cfg AFTER the credentials override so that
	// GetCallerIdentity resolves the effective (possibly assumed-role) account that the
	// CloudWatch client also uses, rather than the base credentials' account.
	if s.stsClient == nil {
		s.stsClient = sts.NewFromConfig(cfg)
	}
	return nil
}

// resolveAccountID resolves the AWS account ID of the active credentials via STS
// GetCallerIdentity and stores it in s.accountID, to be reported as the cloud.account.id
// resource attribute. It runs lazily on scrape (not in start(), which performs no network
// calls) and retries on each scrape until it succeeds: a transient failure logs a warning
// and leaves s.accountID empty for that cycle rather than latching the empty value, so
// metrics are emitted without the attribute only until resolution succeeds.
func (s *cloudWatchMetricsScraper) resolveAccountID(ctx context.Context) {
	if s.accountIDResolved || s.stsClient == nil {
		return
	}
	out, err := s.stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		s.settings.Logger.Warn("unable to resolve AWS account ID via STS GetCallerIdentity; "+
			"will retry on the next scrape. Metrics are emitted without the cloud.account.id "+
			"resource attribute until resolution succeeds.", zap.Error(err))
		return
	}
	s.accountID = aws.ToString(out.Account)
	s.accountIDResolved = true
}

func (s *cloudWatchMetricsScraper) scrape(ctx context.Context) (pmetric.Metrics, error) {
	s.settings.Logger.Debug("scraping CloudWatch metrics", zap.String("region", s.cfg.Region))

	// Resolve the account ID lazily on the first scrape so that start() makes no network calls.
	s.resolveAccountID(ctx)

	var metricsToScrape []MetricQuery
	if s.discovery != nil {
		discovered, err := s.listMetrics(ctx)
		if err != nil {
			return pmetric.NewMetrics(), fmt.Errorf("list metrics: %w", err)
		}
		metricsToScrape = discovered
		s.settings.Logger.Debug("discovered metrics", zap.Int("count", len(metricsToScrape)))
		if len(metricsToScrape) == 0 {
			return pmetric.NewMetrics(), nil
		}
	} else {
		metricsToScrape = s.metrics
	}

	periodSec := int64(s.period.Seconds())
	now := time.Now().UTC()
	// Shift endTime back by delay to account for CloudWatch metric publication latency.
	// Query exactly one collectionInterval worth of data so consecutive scrapes do not overlap.
	endTime := alignTimeToPeriod(now.Add(-s.delay), periodSec)
	startTime := alignTimeToPeriod(endTime.Add(-s.collectionInterval), periodSec)

	md := pmetric.NewMetrics()
	for batchStart := 0; batchStart < len(metricsToScrape); {
		// Build the largest batch whose total sub-query count stays within the API limit.
		queryCount := numSubQueries(metricsToScrape[batchStart])
		batchEnd := batchStart + 1
		for batchEnd < len(metricsToScrape) {
			n := numSubQueries(metricsToScrape[batchEnd])
			if queryCount+n > maxGetMetricDataQueries {
				break
			}
			queryCount += n
			batchEnd++
		}
		batch := metricsToScrape[batchStart:batchEnd]
		batchMd, err := s.pollBatch(ctx, batch, startTime, endTime)
		if err != nil {
			return pmetric.NewMetrics(), err
		}
		batchMd.ResourceMetrics().MoveAndAppendTo(md.ResourceMetrics())
		batchStart = batchEnd
	}
	return md, nil
}

// alignTimeToPeriod rounds t down to the nearest period boundary (in seconds from Unix epoch).
// Per GetMetricData docs, aligning StartTime and EndTime to the metric's Period improves
// performance: https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html
func alignTimeToPeriod(t time.Time, periodSec int64) time.Time {
	if periodSec <= 0 {
		return t
	}
	unix := t.Unix()
	aligned := (unix / periodSec) * periodSec
	return time.Unix(aligned, 0).UTC()
}

// listMetrics discovers metrics via the ListMetrics API, respecting discovery config
// (namespace, metric name, limit, and cross-account options).
//
// When account_identifiers is set, each source account is queried separately because
// ListMetrics filters a single owning account at a time; the limit applies per account.
// When include_linked_accounts is set without identifiers, a single monitoring-account
// sweep discovers metrics across all linked accounts, again capped per account. Otherwise
// the receiver's own account is discovered as before.
func (s *cloudWatchMetricsScraper) listMetrics(ctx context.Context) ([]MetricQuery, error) {
	if len(s.discovery.AccountIdentifiers) > 0 {
		var out []MetricQuery
		for _, account := range s.discovery.AccountIdentifiers {
			qs, err := s.discoverMetrics(ctx, account)
			if err != nil {
				return nil, err
			}
			out = append(out, qs...)
		}
		return out, nil
	}
	return s.discoverMetrics(ctx, "")
}

// discoverMetrics paginates ListMetrics for a single owning account (or, when
// owningAccount is empty and include_linked_accounts is set, a sweep across all linked
// accounts). The discovery limit is enforced per account. Each returned MetricQuery is
// tagged with the account that owns it, or left empty for same-account discovery.
func (s *cloudWatchMetricsScraper) discoverMetrics(ctx context.Context, owningAccount string) ([]MetricQuery, error) {
	includeLinked := s.discovery.IncludeLinkedAccounts != nil && *s.discovery.IncludeLinkedAccounts

	input := &cloudwatch.ListMetricsInput{}
	if f := s.discovery.Filters.Get(); f != nil {
		if f.Namespace != "" {
			input.Namespace = aws.String(f.Namespace)
		}
		if f.MetricName != "" {
			input.MetricName = aws.String(f.MetricName)
		}
	}
	if includeLinked {
		input.IncludeLinkedAccounts = aws.Bool(true)
	}
	if owningAccount != "" {
		input.OwningAccount = aws.String(owningAccount)
	}

	// singleAccount is true whenever every returned metric belongs to one account, so we
	// can stop paginating once the limit is reached. The all-linked sweep must page fully
	// because metrics for under-limit accounts may still appear on later pages.
	singleAccount := owningAccount != "" || !includeLinked

	perAccount := make(map[string]int)
	var out []MetricQuery
	var nextToken *string
	for {
		input.NextToken = nextToken
		resp, err := s.client.ListMetrics(ctx, input)
		if err != nil {
			return nil, err
		}
		dropped := 0
		for i, met := range resp.Metrics {
			var account string
			if singleAccount {
				// Per-identifier discovery uses the requested account; same-account discovery
				// leaves it empty so the receiver's own resolved account is used.
				account = owningAccount
			} else if i >= len(resp.OwningAccounts) || resp.OwningAccounts[i] == "" {
				// Linked-account sweep where AWS did not return an aligned OwningAccounts entry:
				// the metric cannot be attributed to a source account, so drop it rather than
				// misattributing it to the monitoring account.
				dropped++
				continue
			} else {
				// Linked-account sweep: attribute the metric to its source account.
				account = resp.OwningAccounts[i]
			}
			if perAccount[account] >= s.discovery.Limit {
				if singleAccount {
					return out, nil
				}
				continue
			}
			perAccount[account]++
			out = append(out, MetricQuery{
				Namespace:  aws.ToString(met.Namespace),
				MetricName: aws.ToString(met.MetricName),
				Dimensions: dimensionsToMap(met.Dimensions),
				Stats:      s.discovery.Stats,
				AccountID:  account,
			})
		}
		if dropped > 0 {
			s.settings.Logger.Warn("dropped discovered metrics that could not be attributed to a source "+
				"account; ListMetrics OwningAccounts did not align 1:1 with the returned metrics",
				zap.Int("dropped", dropped),
				zap.Int("metrics", len(resp.Metrics)),
				zap.Int("owning_accounts", len(resp.OwningAccounts)))
		}
		nextToken = resp.NextToken
		if nextToken == nil {
			break
		}
	}
	return out, nil
}

func dimensionsToMap(dims []types.Dimension) map[string]string {
	if len(dims) == 0 {
		return nil
	}
	out := make(map[string]string, len(dims))
	for _, d := range dims {
		if d.Name != nil && d.Value != nil {
			out[aws.ToString(d.Name)] = aws.ToString(d.Value)
		}
	}
	return out
}

// metricStats are the four CloudWatch statistics fetched for every metric to build a Summary.
var metricStats = [statsPerMetric]string{"Sum", "SampleCount", "Minimum", "Maximum"}

// numSubQueries returns the number of GetMetricData sub-queries needed for one metric.
// Metrics without an explicit Stats list use all four summary statistics.
func numSubQueries(q MetricQuery) int {
	if len(q.Stats) == 0 {
		return statsPerMetric
	}
	return len(q.Stats)
}

// pollBatch runs GetMetricData for a batch of metrics and returns the converted pdata.Metrics.
// Each metric generates four sub-queries (Sum, SampleCount, Minimum, Maximum) so that the results
// can be combined into an OpenTelemetry Summary metric aligned with the CloudWatch Metric Streams
// OpenTelemetry 1.0.0 format.
// It follows pagination via NextToken to collect all data points for the requested time window.
func (s *cloudWatchMetricsScraper) pollBatch(ctx context.Context, batch []MetricQuery, startTime, endTime time.Time) (pmetric.Metrics, error) {
	totalQueries := 0
	for _, q := range batch {
		totalQueries += numSubQueries(q)
	}
	queries := make([]types.MetricDataQuery, 0, totalQueries)
	for i, q := range batch {
		stats := metricStats[:]
		if len(q.Stats) > 0 {
			stats = q.Stats
		}
		for si, stat := range stats {
			mdq := types.MetricDataQuery{
				Id: aws.String(fmt.Sprintf("q%d_%d", i, si)),
				MetricStat: &types.MetricStat{
					Metric: &types.Metric{
						Namespace:  aws.String(q.Namespace),
						MetricName: aws.String(q.MetricName),
						Dimensions: dimensionsFromMap(q.Dimensions),
					},
					Period: aws.Int32(int32(s.period.Seconds())),
					Stat:   aws.String(stat),
				},
			}
			// In a cross-account setup, fetch the metric from its owning source account.
			if q.AccountID != "" {
				mdq.AccountId = aws.String(q.AccountID)
			}
			queries = append(queries, mdq)
		}
	}

	input := &cloudwatch.GetMetricDataInput{
		StartTime:         aws.Time(startTime),
		EndTime:           aws.Time(endTime),
		MetricDataQueries: queries,
	}

	var allResults []types.MetricDataResult
	for {
		out, err := s.client.GetMetricData(ctx, input)
		if err != nil {
			return pmetric.NewMetrics(), fmt.Errorf("GetMetricData: %w", err)
		}
		allResults = append(allResults, out.MetricDataResults...)
		if out.NextToken == nil {
			break
		}
		input.NextToken = out.NextToken
	}

	withData := 0
	warned := make(map[int]bool)
	for _, r := range allResults {
		if len(r.Values) > 0 {
			withData++
		}
		// Surface non-complete statuses that would otherwise be dropped as empty data.
		// Forbidden in particular signals the metric's account is not accessible (the
		// receiver is not a monitoring account, or the source account is not linked),
		// which is the common cross-account misconfiguration. Deduplicate per metric so
		// a metric's sub-queries do not each emit an identical warning.
		if r.StatusCode == types.StatusCodeForbidden || r.StatusCode == types.StatusCodeInternalError {
			if mi, _, err := parseQueryID(aws.ToString(r.Id)); err == nil && mi >= 0 && mi < len(batch) && !warned[mi] {
				warned[mi] = true
				s.logResultStatus(batch[mi], r)
			}
		}
	}
	s.settings.Logger.Debug("GetMetricData response",
		zap.Int("results", len(allResults)),
		zap.Int("results_with_datapoints", withData),
		zap.Time("start", startTime),
		zap.Time("end", endTime))

	md := s.convertGetMetricDataToPdata(allResults, batch, endTime)
	if md.ResourceMetrics().Len() == 0 {
		if withData > 0 {
			s.settings.Logger.Warn("GetMetricData returned datapoints but conversion produced no resource metrics; check Id matching",
				zap.String("region", s.cfg.Region))
		} else if len(batch) > 0 {
			s.settings.Logger.Debug("No metric data in response; CloudWatch often has 2-5 min delay",
				zap.String("region", s.cfg.Region))
		}
	}
	return md, nil
}

// logResultStatus emits a warning describing a non-complete GetMetricData result so that
// silently-dropped metrics (e.g. a Forbidden cross-account query) are diagnosable.
func (s *cloudWatchMetricsScraper) logResultStatus(q MetricQuery, r types.MetricDataResult) {
	fields := []zap.Field{
		zap.String("status", string(r.StatusCode)),
		zap.String("namespace", q.Namespace),
		zap.String("metric_name", q.MetricName),
		zap.String("account_id", s.effectiveAccountID(q)),
	}
	for _, m := range r.Messages {
		if m.Value != nil {
			fields = append(fields, zap.String("message", aws.ToString(m.Value)))
		}
	}
	s.settings.Logger.Warn("GetMetricData returned a non-complete status for a metric; "+
		"a Forbidden status usually means the receiver is not running in a monitoring account "+
		"or the source account is not linked", fields...)
}

func dimensionsFromMap(d map[string]string) []types.Dimension {
	if len(d) == 0 {
		return nil
	}
	out := make([]types.Dimension, 0, len(d))
	for k, v := range d {
		out = append(out, types.Dimension{Name: aws.String(k), Value: aws.String(v)})
	}
	return out
}

// setResourceAttributes sets the resource-level attributes that identify the AWS source.
// Aligned with the CloudWatch Metric Streams OpenTelemetry 1.0.0 format: cloud.provider,
// cloud.account.id, and cloud.region are set on the resource; namespace and dimensions go
// on data points. cloud.account.id is omitted when accountID is empty.
func (s *cloudWatchMetricsScraper) setResourceAttributes(resource pcommon.Resource, accountID string) {
	attrs := resource.Attributes()
	attrs.PutStr(string(conventions.CloudProviderKey), conventions.CloudProviderAWS.Value.AsString())
	if accountID != "" {
		attrs.PutStr(string(conventions.CloudAccountIDKey), accountID)
	}
	attrs.PutStr(string(conventions.CloudRegionKey), s.cfg.Region)
}

// effectiveAccountID returns the account ID to attribute to a metric query: the query's
// own AccountID when set (cross-account), otherwise the receiver's resolved account ID.
func (s *cloudWatchMetricsScraper) effectiveAccountID(q MetricQuery) string {
	if q.AccountID != "" {
		return q.AccountID
	}
	return s.accountID
}

// parseQueryID parses a sub-query ID of the form "q{metricIdx}_{statIdx}".
func parseQueryID(id string) (metricIdx, statIdx int, err error) {
	trimmed := strings.TrimPrefix(id, "q")
	parts := strings.SplitN(trimmed, "_", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid query ID %q", id)
	}
	metricIdx, err = strconv.Atoi(parts[0])
	if err != nil {
		return metricIdx, statIdx, err
	}
	statIdx, err = strconv.Atoi(parts[1])
	return metricIdx, statIdx, err
}

// convertGetMetricDataToPdata converts GetMetricData results to pdata.Metrics.
//
// When a MetricQuery has no Stats (empty), all four statistics are fetched and combined into an
// OpenTelemetry Summary metric, aligned with the CloudWatch Metric Streams OpenTelemetry 1.0.0 format:
//   - Metric name: "amazonaws.com/{Namespace}/{MetricName}".
//   - Metric type: Summary with count (SampleCount), sum (Sum), min quantile (Minimum), max quantile (Maximum).
//   - Data point attributes: Namespace (string), MetricName (string), Dimensions (kvlist).
//
// When a MetricQuery has explicit Stats, each selected statistic is emitted as a Gauge data point on the
// same metric, with an additional "stat" attribute identifying the statistic.
func (s *cloudWatchMetricsScraper) convertGetMetricDataToPdata(results []types.MetricDataResult, metricsList []MetricQuery, endTime time.Time) pmetric.Metrics {
	// statMaps[metricIdx][statIdx] holds a map from timestamp → value for that metric/stat combination.
	// The inner slice length equals numSubQueries(metricsList[i]).
	type tsMap = map[time.Time]float64
	statMaps := make([][]tsMap, len(metricsList))
	for i, q := range metricsList {
		n := numSubQueries(q)
		statMaps[i] = make([]tsMap, n)
		for j := range statMaps[i] {
			statMaps[i][j] = make(tsMap)
		}
	}

	for _, result := range results {
		if result.Id == nil || len(result.Values) == 0 {
			continue
		}
		metricIdx, statIdx, err := parseQueryID(*result.Id)
		if err != nil || metricIdx < 0 || metricIdx >= len(metricsList) || statIdx < 0 || statIdx >= len(statMaps[metricIdx]) {
			continue
		}
		for j, v := range result.Values {
			ts := endTime
			if j < len(result.Timestamps) {
				ts = result.Timestamps[j]
			}
			statMaps[metricIdx][statIdx][ts] = v
		}
	}

	md := pmetric.NewMetrics()

	// Group metrics into one ResourceMetrics per account ID so that cross-account
	// metrics carry the correct cloud.account.id. In the single-account case all
	// metrics share the receiver's resolved account ID and land in one resource.
	scopeByAccount := make(map[string]pmetric.ScopeMetrics)
	scopeForAccount := func(accountID string) pmetric.ScopeMetrics {
		if sm, ok := scopeByAccount[accountID]; ok {
			return sm
		}
		rm := md.ResourceMetrics().AppendEmpty()
		s.setResourceAttributes(rm.Resource(), accountID)
		sm := rm.ScopeMetrics().AppendEmpty()
		sm.Scope().SetName(metadata.ScopeName)
		scopeByAccount[accountID] = sm
		return sm
	}

	periodNano := pcommon.Timestamp(s.period.Nanoseconds())

	for i, q := range metricsList {
		// Skip metrics with no data across any stat.
		hasData := false
		for _, tsm := range statMaps[i] {
			if len(tsm) > 0 {
				hasData = true
				break
			}
		}
		if !hasData {
			continue
		}

		metric := scopeForAccount(s.effectiveAccountID(q)).Metrics().AppendEmpty()
		metric.SetName("amazonaws.com/" + q.Namespace + "/" + q.MetricName)

		if len(q.Stats) == 0 {
			// Summary mode: combine Sum, SampleCount, Minimum, Maximum into one Summary data point per timestamp.
			tsSet := make(map[time.Time]struct{})
			for _, tsm := range statMaps[i] {
				for ts := range tsm {
					tsSet[ts] = struct{}{}
				}
			}
			timestamps := make([]time.Time, 0, len(tsSet))
			for ts := range tsSet {
				timestamps = append(timestamps, ts)
			}
			sort.Slice(timestamps, func(a, b int) bool { return timestamps[a].Before(timestamps[b]) })

			summary := metric.SetEmptySummary()
			for _, ts := range timestamps {
				dp := summary.DataPoints().AppendEmpty()
				tsNano := pcommon.NewTimestampFromTime(ts)
				dp.SetTimestamp(tsNano)
				if tsNano > periodNano {
					dp.SetStartTimestamp(tsNano - periodNano)
				}
				if count, ok := statMaps[i][statIdxCount][ts]; ok {
					dp.SetCount(uint64(count))
				}
				if sum, ok := statMaps[i][statIdxSum][ts]; ok {
					dp.SetSum(sum)
				}
				if minVal, ok := statMaps[i][statIdxMin][ts]; ok {
					qv := dp.QuantileValues().AppendEmpty()
					qv.SetQuantile(0.0)
					qv.SetValue(minVal)
				}
				if maxVal, ok := statMaps[i][statIdxMax][ts]; ok {
					qv := dp.QuantileValues().AppendEmpty()
					qv.SetQuantile(1.0)
					qv.SetValue(maxVal)
				}
				applyQueryAttrs(dp.Attributes(), q)
			}
		} else {
			// Gauge mode: one Gauge data point per (stat, timestamp), tagged with a "stat" attribute.
			gauge := metric.SetEmptyGauge()
			for si, statName := range q.Stats {
				timestamps := make([]time.Time, 0, len(statMaps[i][si]))
				for ts := range statMaps[i][si] {
					timestamps = append(timestamps, ts)
				}
				sort.Slice(timestamps, func(a, b int) bool { return timestamps[a].Before(timestamps[b]) })
				for _, ts := range timestamps {
					dp := gauge.DataPoints().AppendEmpty()
					tsNano := pcommon.NewTimestampFromTime(ts)
					dp.SetTimestamp(tsNano)
					if tsNano > periodNano {
						dp.SetStartTimestamp(tsNano - periodNano)
					}
					dp.SetDoubleValue(statMaps[i][si][ts])
					applyQueryAttrs(dp.Attributes(), q)
					dp.Attributes().PutStr("stat", statName)
				}
			}
		}
	}

	// No ResourceMetrics are created unless a metric had data, so an empty result
	// means nothing was emitted.
	return md
}

// applyQueryAttrs sets the common data point attributes: Namespace, MetricName, and Dimensions (kvlist).
func applyQueryAttrs(attrs pcommon.Map, q MetricQuery) {
	attrs.PutStr("Namespace", q.Namespace)
	attrs.PutStr("MetricName", q.MetricName)
	if len(q.Dimensions) > 0 {
		dimsMap := attrs.PutEmptyMap("Dimensions")
		for k, v := range q.Dimensions {
			dimsMap.PutStr(k, v)
		}
	}
}
