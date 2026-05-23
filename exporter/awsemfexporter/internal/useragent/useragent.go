// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package useragent // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/internal/useragent"

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const (
	middlewareID = "aws.appsignals.UserAgentHandler"
	// defaultTTL is how long an item in the cache will remain if it has not been re-seen.
	defaultTTL = time.Minute
	// cacheSize is the maximum number of unique telemetry SDK languages that can be stored before one will be evicted.
	cacheSize = 5
	// attrLengthLimit is the maximum length of the language and version that will be used for the user agent.
	attrLengthLimit = 20

	// telemetry.auto.version was deprecated in OTel semconv 1.21.0 in favor of
	// telemetry.distro.version and was dropped from the typed-key API in
	// otelgo semconv 1.40.0. Kept as a fallback for SDKs still emitting it.
	attributeTelemetryAutoVersion = "telemetry.auto.version"

	attributeEBS                = "ci_ebs"
	attributeLocalInstanceStore = "ci_lis"
)

// Map of NVMe feature attributes to their corresponding metric prefixes
var featureMetricPrefixes = map[string]string{
	attributeEBS:                "node_diskio_ebs",
	attributeLocalInstanceStore: "node_diskio_instance_store",
}

type UserAgent struct {
	mu          sync.RWMutex
	prebuiltStr string
	cache       *ttlcache.Cache[string, string]
	featureList map[string]struct{}
}

func NewUserAgent() *UserAgent {
	return newUserAgent(defaultTTL)
}

func newUserAgent(ttl time.Duration) *UserAgent {
	ua := &UserAgent{
		cache: ttlcache.New[string, string](
			ttlcache.WithTTL[string, string](ttl),
			ttlcache.WithCapacity[string, string](cacheSize),
		),
		featureList: make(map[string]struct{}),
	}
	ua.cache.OnEviction(func(context.Context, ttlcache.EvictionReason, *ttlcache.Item[string, string]) {
		ua.build()
	})
	go ua.cache.Start()
	return ua
}

// ID implements middleware.BuildMiddleware.
func (ua *UserAgent) ID() string { return middlewareID }

// HandleBuild implements middleware.BuildMiddleware. Appends the dynamic
// user-agent string to the outgoing request's User-Agent header.
func (ua *UserAgent) HandleBuild(
	ctx context.Context,
	in middleware.BuildInput,
	next middleware.BuildHandler,
) (out middleware.BuildOutput, metadata middleware.Metadata, err error) {
	ua.mu.RLock()
	val := ua.prebuiltStr
	ua.mu.RUnlock()

	if val != "" {
		if req, ok := in.Request.(*smithyhttp.Request); ok {
			cur := req.Header.Get("User-Agent")
			if cur != "" {
				req.Header.Set("User-Agent", cur+" "+val)
			} else {
				req.Header.Set("User-Agent", val)
			}
		}
	}
	return next.HandleBuild(ctx, in)
}

// APIOption returns a smithy stack mutator that registers ua as a Build-step
// middleware. Append it to aws.Config.APIOptions before the AWS service client
// is constructed (APIOptions are snapshotted by NewFromConfig).
func (ua *UserAgent) APIOption() func(*middleware.Stack) error {
	return func(stack *middleware.Stack) error {
		return stack.Build.Add(ua, middleware.After)
	}
}

// Process takes the telemetry SDK language and version and adds them to the cache. If it already exists in the
// cache and has the same value, extends the TTL. If not, then it sets it and rebuilds the user agent string.
func (ua *UserAgent) Process(labels map[string]string) {
	language := labels[string(semconv.TelemetrySDKLanguageKey)]
	version := labels[string(semconv.TelemetryDistroVersionKey)]
	if version == "" {
		version = labels[attributeTelemetryAutoVersion]
	}
	if language != "" && version != "" {
		language = truncate(language, attrLengthLimit)
		version = truncate(version, attrLengthLimit)
		value := ua.cache.Get(language)
		if value == nil || value.Value() != version {
			ua.cache.Set(language, version, ttlcache.DefaultTTL)
			ua.build()
		}
	}
}

// ProcessMetrics checks metric names for specific patterns and updates user agent accordingly
func (ua *UserAgent) ProcessMetrics(metrics pmetric.Metrics) {
	// Check if all NVME features are already detected
	allFeaturesFound := true
	for feature := range featureMetricPrefixes {
		if _, exists := ua.featureList[feature]; !exists {
			allFeaturesFound = false
			break
		}
	}
	if allFeaturesFound {
		return
	}

	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				for feature, prefix := range featureMetricPrefixes {
					if strings.HasPrefix(metric.Name(), prefix) {
						if _, exists := ua.featureList[feature]; !exists {
							ua.featureList[feature] = struct{}{}
						}
					}
				}
			}
		}
	}
	ua.build()
}

// build the user agent string from the items in the cache. Format is telemetry-sdk (<lang1>/<ver1>;<lang2>/<ver2>).
func (ua *UserAgent) build() {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	var items []string
	for _, item := range ua.cache.Items() {
		items = append(items, formatStr(item.Key(), item.Value()))
	}
	ua.prebuiltStr = ""
	if len(items) > 0 {
		sort.Strings(items)
		ua.prebuiltStr = fmt.Sprintf("telemetry-sdk (%s)", strings.Join(items, ";"))
	}

	if len(ua.featureList) > 0 {
		if ua.prebuiltStr != "" {
			ua.prebuiltStr += " "
		}
		var metricTypes []string
		for metricType := range ua.featureList {
			metricTypes = append(metricTypes, metricType)
		}
		sort.Strings(metricTypes)
		ua.prebuiltStr += fmt.Sprintf("feature:(%s)", strings.Join(metricTypes, " "))
	}
}

// formatStr formats the telemetry SDK language and version into the user agent format.
func formatStr(language, version string) string {
	return language + "/" + version
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return strings.TrimSpace(s[:n])
	}
	return s
}
