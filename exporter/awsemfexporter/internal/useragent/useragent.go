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

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/jellydator/ttlcache/v3"
	"go.opentelemetry.io/collector/pdata/pmetric"
	semconv "go.opentelemetry.io/collector/semconv/v1.18.0"
)

const (
	handlerName = "aws.appsignals.UserAgentHandler"
	// defaultTTL is how long an item in the cache will remain if it has not been re-seen.
	defaultTTL = time.Minute
	// cacheSize is the maximum number of unique telemetry SDK languages that can be stored before one will be evicted.
	cacheSize = 5
	// attrLengthLimit is the maximum length of the language and version that will be used for the user agent.
	attrLengthLimit = 20
	// TODO: Available in semconv/v1.21.0+. Replace after collector dependency is v0.91.0+.
	attributeTelemetryDistroVersion = "telemetry.distro.version"
	attributeEBS                    = "ci_ebs"
	ebsMetricPrefix                 = "node_diskio_ebs"
)

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

// Handler creates a named handler with the UserAgent's handle function.
func (ua *UserAgent) Handler() request.NamedHandler {
	return request.NamedHandler{
		Name: handlerName,
		Fn:   ua.handle,
	}
}

// handle adds the pre-built user agent string to the user agent header.
func (ua *UserAgent) handle(r *request.Request) {
	ua.mu.RLock()
	defer ua.mu.RUnlock()
	currentUA := r.HTTPRequest.Header.Get("User-Agent")
	if !strings.Contains(currentUA, ua.prebuiltStr) {
		request.AddToUserAgent(r, ua.prebuiltStr)
	}
}

// Process takes the telemetry SDK language and version and adds them to the cache. If it already exists in the
// cache and has the same value, extends the TTL. If not, then it sets it and rebuilds the user agent string.
func (ua *UserAgent) Process(labels map[string]string) {
	language := labels[semconv.AttributeTelemetrySDKLanguage]
	version := labels[attributeTelemetryDistroVersion]
	if version == "" {
		version = labels[semconv.AttributeTelemetryAutoVersion]
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
	if _, exists := ua.featureList[attributeEBS]; exists {
		return
	}
	rms := metrics.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		ilms := rms.At(i).ScopeMetrics()
		for j := 0; j < ilms.Len(); j++ {
			ms := ilms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				metric := ms.At(k)
				if strings.HasPrefix(metric.Name(), ebsMetricPrefix) {
					ua.featureList[attributeEBS] = struct{}{}
					ua.build()
					return
				}
			}
		}
	}
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
