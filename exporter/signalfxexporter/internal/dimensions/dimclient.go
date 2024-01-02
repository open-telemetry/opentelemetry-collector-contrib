// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dimensions // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/dimensions"

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/config/configopaque"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter/internal/translation/dpfilters"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/common/sanitize"
)

// DimensionClient sends updates to dimensions to the SignalFx API
// This is a port of https://github.com/signalfx/signalfx-agent/blob/main/pkg/core/writer/dimensions/client.go
// with the only major difference being deduplication of dimension
// updates are currently not done by this port.
type DimensionClient struct {
	sync.RWMutex
	ctx           context.Context
	Token         configopaque.String
	APIURL        *url.URL
	client        *http.Client
	requestSender *ReqSender
	// How long to wait for property updates to be sent once they are
	// generated.  Any duplicate updates to the same dimension within this time
	// frame will result in the latest property set being sent.  This helps
	// prevent spurious updates that get immediately overwritten by very flappy
	// property generation.
	sendDelay time.Duration
	// Set of dims that have been queued up for sending.  Use map to quickly
	// look up in case we need to replace due to flappy prop generation.
	delayedSet map[DimensionKey]*DimensionUpdate
	// Queue of dimensions to update.  The ordering should never change once
	// put in the queue so no need for heap/priority queue.
	delayedQueue chan *queuedDimension
	// For easier unit testing
	now func() time.Time

	logUpdates       bool
	logger           *zap.Logger
	metricsConverter translation.MetricsConverter
	// ExcludeProperties will filter DimensionUpdate content to not submit undesired metadata.
	ExcludeProperties []dpfilters.PropertyFilter
}

type queuedDimension struct {
	*DimensionUpdate
	TimeToSend time.Time
}

type DimensionClientOptions struct {
	Token        configopaque.String
	APIURL       *url.URL
	APITLSConfig *tls.Config
	LogUpdates   bool
	Logger       *zap.Logger
	SendDelay    time.Duration
	// In case of having issues sending dimension updates to SignalFx,
	// buffer a fixed number of updates.
	MaxBuffered         int
	MetricsConverter    translation.MetricsConverter
	ExcludeProperties   []dpfilters.PropertyFilter
	MaxConnsPerHost     int
	MaxIdleConns        int
	MaxIdleConnsPerHost int
	IdleConnTimeout     time.Duration
	Timeout             time.Duration
}

// NewDimensionClient returns a new client
func NewDimensionClient(ctx context.Context, options DimensionClientOptions) *DimensionClient {
	client := &http.Client{
		Timeout: options.Timeout,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).DialContext,
			MaxConnsPerHost:     options.MaxConnsPerHost,
			MaxIdleConns:        options.MaxIdleConns,
			MaxIdleConnsPerHost: options.MaxIdleConnsPerHost,
			IdleConnTimeout:     options.IdleConnTimeout,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig:     options.APITLSConfig,
		},
	}
	sender := NewReqSender(ctx, client, 20, map[string]string{"client": "dimension"})

	return &DimensionClient{
		ctx:               ctx,
		Token:             options.Token,
		APIURL:            options.APIURL,
		sendDelay:         options.SendDelay,
		delayedSet:        make(map[DimensionKey]*DimensionUpdate),
		delayedQueue:      make(chan *queuedDimension, options.MaxBuffered),
		requestSender:     sender,
		client:            client,
		now:               time.Now,
		logger:            options.Logger,
		logUpdates:        options.LogUpdates,
		metricsConverter:  options.MetricsConverter,
		ExcludeProperties: options.ExcludeProperties,
	}
}

// Start the client's processing queue
func (dc *DimensionClient) Start() {
	go dc.processQueue()
}

// acceptDimension to be sent to the API.  This will return fairly quickly and
// won't block. If the buffer is full, the dim update will be dropped.
func (dc *DimensionClient) acceptDimension(dimUpdate *DimensionUpdate) error {
	if dimUpdate = dc.filterDimensionUpdate(dimUpdate); dimUpdate == nil {
		return nil
	}

	dc.Lock()
	defer dc.Unlock()

	if delayedDimUpdate := dc.delayedSet[dimUpdate.Key()]; delayedDimUpdate != nil {
		if !reflect.DeepEqual(delayedDimUpdate, dimUpdate) {
			// Merge the latest updates into existing one.
			delayedDimUpdate.Properties = mergeProperties(delayedDimUpdate.Properties, dimUpdate.Properties)
			delayedDimUpdate.Tags = mergeTags(delayedDimUpdate.Tags, dimUpdate.Tags)
		}
	} else {
		dc.delayedSet[dimUpdate.Key()] = dimUpdate
		select {
		case dc.delayedQueue <- &queuedDimension{
			DimensionUpdate: dimUpdate,
			TimeToSend:      dc.now().Add(dc.sendDelay),
		}:
			break
		default:
			return errors.New("dropped dimension update, propertiesMaxBuffered exceeded")
		}
	}

	return nil
}

// mergeProperties merges 2 or more maps of properties. This method gives
// precedence to values of properties in later maps. i.e., if more than one
// map has the same key, the last value seen will be the effective value in
// the output.
func mergeProperties(propMaps ...map[string]*string) map[string]*string {
	out := map[string]*string{}
	for _, propMap := range propMaps {
		for k, v := range propMap {
			out[k] = v
		}
	}
	return out
}

// mergeTags merges 2 or more sets of tags. This method gives precedence to
// tags seen in later sets. i.e., if more than one set has the same tag, the
// last value seen will be the effective value in the output.
func mergeTags(tagSets ...map[string]bool) map[string]bool {
	out := map[string]bool{}
	for _, tagSet := range tagSets {
		for k, v := range tagSet {
			out[k] = v
		}
	}
	return out
}

func (dc *DimensionClient) processQueue() {
	for {
		select {
		case <-dc.ctx.Done():
			return
		case delayedDimUpdate := <-dc.delayedQueue:
			now := dc.now()
			if now.Before(delayedDimUpdate.TimeToSend) {
				// dims are always in the channel in order of TimeToSend
				time.Sleep(delayedDimUpdate.TimeToSend.Sub(now))
			}

			dc.Lock()
			delete(dc.delayedSet, delayedDimUpdate.Key())
			dc.Unlock()

			if err := dc.handleDimensionUpdate(delayedDimUpdate.DimensionUpdate); err != nil {
				dc.logger.Error(
					"Could not send dimension update",
					zap.Error(err),
					zap.String("dimensionUpdate", delayedDimUpdate.String()),
				)
			}
		}
	}
}

// handleDimensionUpdate will set custom properties on a specific dimension value.
func (dc *DimensionClient) handleDimensionUpdate(dimUpdate *DimensionUpdate) error {
	var (
		req *http.Request
		err error
	)

	req, err = dc.makePatchRequest(dimUpdate)

	if err != nil {
		return err
	}

	req = req.WithContext(
		context.WithValue(req.Context(), RequestFailedCallbackKey, RequestFailedCallback(func(statusCode int, err error) {
			if statusCode >= 400 && statusCode < 500 && statusCode != 404 {
				dc.logger.Error(
					"Unable to update dimension, not retrying",
					zap.Error(err),
					zap.String("URL", sanitize.URL(req.URL)),
					zap.String("dimensionUpdate", dimUpdate.String()),
					zap.Int("statusCode", statusCode),
				)

				// Don't retry if it is a 4xx error (except 404) since these
				// imply an input/auth error, which is not going to be remedied
				// by retrying.
				// 404 errors are special because they can occur due to races
				// within the dimension patch endpoint.
				return
			}

			dc.logger.Error(
				"Unable to update dimension, retrying",
				zap.Error(err),
				zap.String("URL", sanitize.URL(req.URL)),
				zap.String("dimensionUpdate", dimUpdate.String()),
				zap.Int("statusCode", statusCode),
			)
			// The retry is meant to provide some measure of robustness against
			// temporary API failures.  If the API is down for significant
			// periods of time, dimension updates will probably eventually back
			// up beyond PropertiesMaxBuffered and start dropping.
			if err := dc.acceptDimension(dimUpdate); err != nil {
				dc.logger.Error(
					"Failed to retry dimension update",
					zap.Error(err),
					zap.String("URL", sanitize.URL(req.URL)),
					zap.String("dimensionUpdate", dimUpdate.String()),
					zap.Int("statusCode", statusCode),
				)
			}
		})))

	req = req.WithContext(
		context.WithValue(req.Context(), RequestSuccessCallbackKey, RequestSuccessCallback(func([]byte) {
			if dc.logUpdates {
				dc.logger.Info(
					"Updated dimension",
					zap.String("dimensionUpdate", dimUpdate.String()),
				)
			}
		})))

	dc.requestSender.Send(req)

	return nil
}

func (dc *DimensionClient) makeDimURL(key, value string) (*url.URL, error) {
	url, err := dc.APIURL.Parse(fmt.Sprintf("/v2/dimension/%s/%s", url.PathEscape(key), url.PathEscape(value)))
	if err != nil {
		return nil, fmt.Errorf("could not construct dimension property PATCH URL with %s / %s: %w", key, value, err)
	}

	return url, nil
}

func (dc *DimensionClient) makePatchRequest(dim *DimensionUpdate) (*http.Request, error) {
	var (
		tagsToAdd    []string
		tagsToRemove []string
	)

	for tag, shouldAdd := range dim.Tags {
		if shouldAdd {
			tagsToAdd = append(tagsToAdd, tag)
		} else {
			tagsToRemove = append(tagsToRemove, tag)
		}
	}

	json, err := json.Marshal(map[string]any{
		"customProperties": dim.Properties,
		"tags":             tagsToAdd,
		"tagsToRemove":     tagsToRemove,
	})
	if err != nil {
		return nil, err
	}

	url, err := dc.makeDimURL(dim.Name, dim.Value)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		"PATCH",
		strings.TrimRight(url.String(), "/")+"/_/sfxagent",
		bytes.NewReader(json))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("X-SF-TOKEN", string(dc.Token))

	return req, nil
}

func (dc *DimensionClient) filterDimensionUpdate(update *DimensionUpdate) *DimensionUpdate {
	for _, excludeRule := range dc.ExcludeProperties {
		if excludeRule.DimensionName.Matches(update.Name) && excludeRule.DimensionValue.Matches(update.Value) {
			for k, v := range update.Properties {
				if excludeRule.PropertyName.Matches(k) {
					vVal := ""
					if v != nil {
						vVal = *v
					}
					if excludeRule.PropertyValue.Matches(vVal) {
						delete(update.Properties, k)
					}
				}
			}
		}
	}

	// Prevent needless dimension updates if all content has been filtered.
	// Based on https://github.com/signalfx/signalfx-agent/blob/a10f69ec6b95d7426adaf639773628fa034628b8/pkg/core/propfilters/dimfilter.go#L95
	if len(update.Properties) == 0 && len(update.Tags) == 0 {
		return nil
	}

	return update
}
