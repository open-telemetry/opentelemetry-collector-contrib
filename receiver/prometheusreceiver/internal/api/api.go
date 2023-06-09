// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0
//
// Copyright 2016 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver/internal/api"

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/scrape"
	promapi "github.com/prometheus/prometheus/web/api/v1"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

// TargetRetriever provides the list of active/dropped targets to scrape or not.
type TargetRetriever interface {
	TargetsActive() map[string][]*scrape.Target
	TargetsDropped() map[string][]*scrape.Target
}

type Config struct {
	ServerConfig *confighttp.HTTPServerSettings `mapstructure:"server"`
	// ExternalURL is used to construct externally-accessible scrape URLs
	// for targets on localhost
	ExternalURL string `mapstructure:"external_url"`
}

type response struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
}

type API struct {
	retriever   TargetRetriever
	listener    net.Listener
	server      *http.Server
	logger      *zap.Logger
	externalURL *url.URL
}

func NewAPI(cfg *Config, logger *zap.Logger, host component.Host, settings component.TelemetrySettings, ret TargetRetriever) (*API, error) {
	api := &API{retriever: ret, logger: logger.Named("api")}
	mux := http.NewServeMux()
	mux.HandleFunc("/targets", api.handle(api.targets))
	var err error
	api.listener, err = cfg.ServerConfig.ToListener()
	if err != nil {
		logger.Error("failure creating API listener", zap.Error(err))
		return nil, err
	}

	api.server, err = cfg.ServerConfig.ToServer(host, settings, mux)
	if err != nil {
		logger.Error("failure creating API server", zap.Error(err))
		return nil, err
	}

	if cfg.ExternalURL != "" {
		externalURL, err := url.Parse(cfg.ExternalURL)
		if err != nil {
			logger.Error("unable to parse external URL", zap.Error(err))
			return nil, err
		}
		api.externalURL = externalURL
	} else {
		host, err := os.Hostname()
		if err != nil {
			logger.Error("unable to get hostname", zap.Error(err))
			return nil, err
		}
		api.externalURL = &url.URL{
			Scheme: "http",
			Host:   host,
		}
	}

	return api, nil
}

func (a *API) Run() {
	if a.server == nil {
		a.logger.Error("no API server configured")
		return
	}
	go func() {
		err := a.server.Serve(a.listener)
		if err != nil {
			a.logger.Error("error serving API", zap.Error(err))
		}
	}()
}

func (a *API) Shutdown(ctx context.Context) error {
	if a.server == nil {
		return nil
	}
	return a.server.Shutdown(ctx)
}

// sanitizeSplitHostPort acts like net.SplitHostPort.
// Additionally, if there is no port in the host passed as input, we return the
// original host, making sure that IPv6 addresses are not surrounded by square
// brackets.
//
// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api.go#L887
func sanitizeSplitHostPort(input string) (string, string, error) {
	host, port, err := net.SplitHostPort(input)
	if err != nil && strings.HasSuffix(err.Error(), "missing port in address") {
		var errWithPort error
		host, _, errWithPort = net.SplitHostPort(input + ":80")
		if errWithPort == nil {
			err = nil
		}
	}
	return host, port, err
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api.go#L899
// with adaptations as required
func (a *API) getGlobalURL(u *url.URL) (*url.URL, error) {
	host, port, err := sanitizeSplitHostPort(u.Host)
	if err != nil {
		return u, err
	}

	for _, lhr := range promapi.LocalhostRepresentations {
		if host == lhr {
			_, ownPort, err := net.SplitHostPort(a.listener.Addr().String())
			if err != nil {
				return u, err
			}

			if port == ownPort {
				// Only in the case where the target is on localhost and its port is
				// the same as the one we're listening on, we know for sure that
				// we're monitoring our own process and that we need to change the
				// scheme, hostname, and port to the externally reachable ones as
				// well. We shouldn't need to touch the path at all, since if a
				// path prefix is defined, the path under which we scrape ourselves
				// should already contain the prefix.
				u.Scheme = a.externalURL.Scheme
				u.Host = a.externalURL.Host
			} else {
				// Otherwise, we only know that localhost is not reachable
				// externally, so we replace only the hostname by the one in the
				// external URL. It could be the wrong hostname for the service on
				// this port, but it's still the best possible guess.
				host, _, err := sanitizeSplitHostPort(a.externalURL.Host)
				if err != nil {
					return u, err
				}
				u.Host = host
				if port != "" {
					u.Host = net.JoinHostPort(u.Host, port)
				}
			}
			break
		}
	}

	return u, nil
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api.go#L950
// with adaptations as required
func (a *API) targets(req *http.Request) response {
	sortKeys := func(targets map[string][]*scrape.Target) ([]string, int) {
		var n int
		keys := make([]string, 0, len(targets))
		for k := range targets {
			keys = append(keys, k)
			n += len(targets[k])
		}
		slices.Sort(keys)
		return keys, n
	}

	scrapePool := req.URL.Query().Get("scrapePool")
	state := strings.ToLower(req.URL.Query().Get("state"))
	showActive := state == "" || state == "any" || state == "active"
	showDropped := state == "" || state == "any" || state == "dropped"
	res := &promapi.TargetDiscovery{}

	if showActive {
		targetsActive := a.retriever.TargetsActive()
		activeKeys, numTargets := sortKeys(targetsActive)
		res.ActiveTargets = make([]*promapi.Target, 0, numTargets)

		for _, key := range activeKeys {
			if scrapePool != "" && key != scrapePool {
				continue
			}
			for _, target := range targetsActive[key] {
				lastErrStr := ""
				lastErr := target.LastError()
				if lastErr != nil {
					lastErrStr = lastErr.Error()
				}

				globalURL, err := a.getGlobalURL(target.URL())

				res.ActiveTargets = append(res.ActiveTargets, &promapi.Target{
					DiscoveredLabels: target.DiscoveredLabels().Map(),
					Labels:           target.Labels().Map(),
					ScrapePool:       key,
					ScrapeURL:        target.URL().String(),
					GlobalURL:        globalURL.String(),
					LastError: func() string {
						switch {
						case err == nil && lastErrStr == "":
							return ""
						case err != nil:
							return fmt.Sprintf("%s: %s", lastErrStr, err.Error())
						default:
							return lastErrStr
						}
					}(),
					LastScrape:         target.LastScrape(),
					LastScrapeDuration: target.LastScrapeDuration().Seconds(),
					Health:             target.Health(),
					ScrapeInterval:     target.GetValue(model.ScrapeIntervalLabel),
					ScrapeTimeout:      target.GetValue(model.ScrapeTimeoutLabel),
				})
			}
		}
	} else {
		res.ActiveTargets = []*promapi.Target{}
	}

	if showDropped {
		targetsDropped := a.retriever.TargetsDropped()
		droppedKeys, numTargets := sortKeys(targetsDropped)
		res.DroppedTargets = make([]*promapi.DroppedTarget, 0, numTargets)
		for _, key := range droppedKeys {
			if scrapePool != "" && key != scrapePool {
				continue
			}
			for _, target := range targetsDropped[key] {
				res.DroppedTargets = append(res.DroppedTargets, &promapi.DroppedTarget{
					DiscoveredLabels: target.DiscoveredLabels().Map(),
				})
			}
		}
	} else {
		res.DroppedTargets = []*promapi.DroppedTarget{}
	}

	return response{
		Status: "success",
		Data:   res,
	}
}

// borrowed from https://github.com/prometheus/prometheus/blob/344c8ff97ce261dbaaf2720f1e5164a8fee19184/web/api/v1/api.go#L1630
// with adaptations as required
func (a *API) respond(w http.ResponseWriter, data any) {
	json := jsoniter.ConfigCompatibleWithStandardLibrary
	res, err := json.Marshal(&response{Status: "success", Data: data})
	if err != nil {
		a.logger.Error("error marshaling JSON response", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if n, err := w.Write(res); err != nil {
		a.logger.Error("error writing response", zap.Error(err), zap.Int("bytesWritten", n))
	}
}

func (a *API) handle(f func(r *http.Request) response) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		res := f(req)
		a.respond(w, res.Data)
	}
}
