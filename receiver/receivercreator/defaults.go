// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/go-viper/mapstructure/v2"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/scraper/xscraper"
	"go.opentelemetry.io/collector/service/hostcapabilities"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

// resolveDefaultScrapers builds a receiver template for every scraper listed under
// discovery::defaults::scrapers from the default discovery rule of its factory.
// Scrapers that have an explicit receiver template of the same type are skipped.
func resolveDefaultScrapers(cfg *Config, h component.Host, logger *zap.Logger) ([]receiverTemplate, error) {
	if len(cfg.Discovery.Defaults.Scrapers) == 0 {
		return nil, nil
	}
	sfHost, ok := h.(hostcapabilities.ScraperFactory)
	if !ok {
		return nil, errors.New("discovery::defaults::scrapers requires a host that provides scraper factories")
	}

	templateTypes := map[component.Type]bool{}
	for _, t := range cfg.receiverTemplates {
		templateTypes[t.receiverConfig.id.Type()] = true
	}

	var templates []receiverTemplate
	var errs error
	for _, name := range slices.Sorted(maps.Keys(cfg.Discovery.Defaults.Scrapers)) {
		typ, err := component.NewType(name)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("discovery::defaults::scrapers: invalid scraper type %q: %w", name, err))
			continue
		}
		if templateTypes[typ] {
			logger.Info("skipping default discovery for scraper with an explicit receiver template", zap.String("scraper", name))
			continue
		}
		t, err := newDefaultScraperTemplate(sfHost, typ, cfg.Discovery.Defaults.Scrapers[name])
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("discovery::defaults::scrapers::%s: %w", name, err))
			continue
		}
		templates = append(templates, t)
	}
	return templates, errs
}

func newDefaultScraperTemplate(h hostcapabilities.ScraperFactory, typ component.Type, dc DefaultComponentConfig) (receiverTemplate, error) {
	f := h.GetScraperFactory(typ)
	if f == nil {
		return receiverTemplate{}, errors.New("scraper is not in the build")
	}
	xf, ok := f.(xscraper.Factory)
	if !ok || xf.DiscoveryRule().IsEmpty() {
		return receiverTemplate{}, errors.New("scraper has no default discovery rule")
	}
	dr := xf.DiscoveryRule()

	endpointRules := make(map[observer.EndpointType]rule, len(dr.Endpoints))
	for et, er := range dr.Endpoints {
		r, err := newRule(fmt.Sprintf("type == %q and (%s)", et, er.Rule))
		if err != nil {
			return receiverTemplate{}, fmt.Errorf("invalid rule for endpoint type %q: %w", et, err)
		}
		endpointRules[observer.EndpointType(et)] = r
	}

	// User overrides are merged over the rule's default config.
	conf := confmap.NewFromStringMap(dr.DefaultConfig)
	if err := conf.Merge(confmap.NewFromStringMap(dc.Config)); err != nil {
		return receiverTemplate{}, fmt.Errorf("failed to merge config: %w", err)
	}

	t, err := newReceiverTemplate(typ.String(), conf.ToStringMap())
	if err != nil {
		return receiverTemplate{}, err
	}
	t.signals = receiverSignals{metrics: true}
	t.receiverConfig.scraperFactory = xf
	t.endpointRules = endpointRules
	return t, nil
}

// matches reports whether the template's rule matches the endpoint.
func (t *receiverTemplate) matches(e observer.Endpoint, env observer.EndpointEnv) (bool, error) {
	if t.endpointRules == nil {
		return t.rule.eval(env)
	}
	r, ok := t.endpointRules[e.Details.Type()]
	if !ok {
		return false, nil
	}
	return r.eval(env)
}

// claimedByMetricsHints reports whether the endpoint's metrics hints claim it, so that
// default discovery must not start anything for it. The pod's own annotations always
// count. default_annotations only count when hints are enabled.
func claimedByMetricsHints(cfg DiscoveryConfig, env observer.EndpointEnv) bool {
	if getStringEnv(env, "type") != string(observer.PortType) {
		return false
	}
	var p observer.Port
	if err := mapstructure.Decode(env, &p); err != nil || p.Port == 0 {
		return false
	}
	annotations := p.Pod.Annotations
	if cfg.Enabled {
		annotations = mergeAnnotations(annotations, cfg.DefaultAnnotations)
	}
	port := fmt.Sprint(p.Port)
	enabled, found := getHintAnnotation(annotations, otelMetricsHints, discoveryEnabledHint, port)
	if !found {
		return false
	}
	if enabled == "false" {
		return true
	}
	scraperName, _ := getHintAnnotation(annotations, otelMetricsHints, scraperHint, port)
	return enabled == "true" && scraperName != ""
}
