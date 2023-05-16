// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/scrape"
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

// Target has the information for one target.
type Target struct {
	// Labels before any processing.
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
	// Any labels that are added to this target and its metrics.
	Labels map[string]string `json:"labels"`

	ScrapePool string `json:"scrapePool"`
	ScrapeURL  string `json:"scrapeUrl"`
	GlobalURL  string `json:"globalUrl"`

	LastError          string              `json:"lastError"`
	LastScrape         time.Time           `json:"lastScrape"`
	LastScrapeDuration float64             `json:"lastScrapeDuration"`
	Health             scrape.TargetHealth `json:"health"`

	ScrapeInterval string `json:"scrapeInterval"`
	ScrapeTimeout  string `json:"scrapeTimeout"`
}

// DroppedTarget has the information for one target that was dropped during relabelling.
type DroppedTarget struct {
	// Labels before any processing.
	DiscoveredLabels map[string]string `json:"discoveredLabels"`
}

// TargetDiscovery has all the active targets.
type TargetDiscovery struct {
	ActiveTargets  []*Target        `json:"activeTargets"`
	DroppedTargets []*DroppedTarget `json:"droppedTargets"`
}

type API struct {
	retriever TargetRetriever
	listener  net.Listener
	server    *http.Server
	done      chan struct{}
}

func NewAPI(cfg *confighttp.HTTPServerSettings, logger *zap.Logger, host component.Host, settings component.TelemetrySettings, ret TargetRetriever) (*API, error) {
	api := &API{retriever: ret, done: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("/targets", api.targets)
	var err error
	api.listener, err = cfg.ToListener()
	if err != nil {
		logger.Error("failure creating API listener", zap.Error(err))
		return nil, err
	}

	api.server, err = cfg.ToServer(host, settings, mux)
	if err != nil {
		logger.Error("failure creating API server", zap.Error(err))
		return nil, err
	}

	return api, nil
}

func (a *API) Run() {
	go func() {
		a.server.Serve(a.listener)
	}()
}

func (a *API) Shutdown(ctx context.Context) error {
	return a.server.Shutdown(ctx)
}

func (a *API) targets(w http.ResponseWriter, req *http.Request) {
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
	res := &TargetDiscovery{}

	if showActive {
		targetsActive := a.retriever.TargetsActive()
		activeKeys, numTargets := sortKeys(targetsActive)
		res.ActiveTargets = make([]*Target, 0, numTargets)

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

				res.ActiveTargets = append(res.ActiveTargets, &Target{
					DiscoveredLabels:   target.DiscoveredLabels().Map(),
					Labels:             target.Labels().Map(),
					ScrapePool:         key,
					ScrapeURL:          target.URL().String(),
					LastError:          lastErrStr,
					LastScrape:         target.LastScrape(),
					LastScrapeDuration: target.LastScrapeDuration().Seconds(),
					Health:             target.Health(),
					ScrapeInterval:     target.GetValue(model.ScrapeIntervalLabel),
					ScrapeTimeout:      target.GetValue(model.ScrapeTimeoutLabel),
				})
			}
		}
	} else {
		res.ActiveTargets = []*Target{}
	}
	if showDropped {
		targetsDropped := a.retriever.TargetsDropped()
		droppedKeys, numTargets := sortKeys(targetsDropped)
		res.DroppedTargets = make([]*DroppedTarget, 0, numTargets)
		for _, key := range droppedKeys {
			if scrapePool != "" && key != scrapePool {
				continue
			}
			for _, target := range targetsDropped[key] {
				res.DroppedTargets = append(res.DroppedTargets, &DroppedTarget{
					DiscoveredLabels: target.DiscoveredLabels().Map(),
				})
			}
		}
	} else {
		res.DroppedTargets = []*DroppedTarget{}
	}
	b, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("content-type", "application/json")
	w.Write(b)
}
