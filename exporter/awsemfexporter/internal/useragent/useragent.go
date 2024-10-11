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
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
)

const (
	handlerName = "aws.awsemfexporter.UserAgentHandler"
	// defaultTTL is how long an item in the cache will remain if it has not been re-seen.
	defaultTTL = time.Minute
	// cacheSize is the maximum number of unique telemetry SDK languages that can be stored before one will be evicted.
	cacheSize = 10
	// attrLengthLimit is the maximum length of the language and version that will be used for the user agent.
	attrLengthLimit = 20
	// Relic attribute from before semconv/v1.21.0. Replaced with "telemetry.distro.version", but old SDKs may still be using it
	attributeTelemetryAutoVersion = "telemetry.auto.version"
)

type UserAgent struct {
	mu          sync.RWMutex
	prebuiltStr string
	cache       *ttlcache.Cache[string, map[string]struct{}]
}

func NewUserAgent() *UserAgent {
	return newUserAgent(defaultTTL)
}

func newUserAgent(ttl time.Duration) *UserAgent {
	ua := &UserAgent{
		cache: ttlcache.New[string, map[string]struct{}](
			ttlcache.WithTTL[string, map[string]struct{}](ttl),
			ttlcache.WithCapacity[string, map[string]struct{}](cacheSize),
		),
	}
	ua.cache.OnEviction(func(context.Context, ttlcache.EvictionReason, *ttlcache.Item[string, map[string]struct{}]) {
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
	request.AddToUserAgent(r, ua.prebuiltStr)
}

// Process takes the telemetry SDK language and version and adds them to the cache. If the language already exists in the
// cache with a different version, then add the new version to the version map. If not, then create a new version map. 
func (ua *UserAgent) Process(labels map[string]string) {
	language := labels[semconv.AttributeTelemetrySDKLanguage]
	version := labels[semconv.AttributeTelemetryDistroVersion]
	if version == "" {
		version = labels[attributeTelemetryAutoVersion]
	}
	if language != "" && version != "" {
		language = truncate(language, attrLengthLimit)
		version = truncate(version, attrLengthLimit)
		value := ua.cache.Get(language)
		valueMap := make(map[string]struct{})
		if value != nil {
			valueMap = value.Value()
		}
		
		if _, exists := valueMap[version]; !exists {
			valueMap[version] = struct{}{}
			ua.cache.Set(language, valueMap, ttlcache.DefaultTTL)
			ua.build()
		}
	}
}

// build the user agent string from the items in the cache. Format is telemetry-sdk (<lang1>/<ver1>,<ver2>;<lang2>/<ver2>).
func (ua *UserAgent) build() {
	ua.mu.Lock()
	defer ua.mu.Unlock()
	var items []string
	for _, item := range ua.cache.Items() {
		value := item.Value()
		var versionStr string
		for version := range value {
			if versionStr != "" {
				versionStr += "," 
			}
			versionStr += version
		}
		items = append(items, formatStr(item.Key(), versionStr))
	}
	ua.prebuiltStr = ""
	if len(items) > 0 {
		sort.Strings(items)
		ua.prebuiltStr = fmt.Sprintf("telemetry-sdk (%s)", strings.Join(items, ";"))
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

func (ua *UserAgent) ShutDown() {
	ua.cache.Stop()
}
