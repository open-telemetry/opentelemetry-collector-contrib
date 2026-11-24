// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package prometheusremotewrite // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite"

import (
	"slices"
	"strings"

	"github.com/prometheus/common/model"
	"github.com/prometheus/otlptranslator"
	"github.com/prometheus/prometheus/prompb"
	writev2 "github.com/prometheus/prometheus/prompb/io/prometheus/write/v2"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/multierr"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type entityTargetKey struct {
	job      string
	instance string
}

type entityTarget struct {
	resource  pcommon.Resource
	job       string
	instance  string
	timestamp pcommon.Timestamp
}

func recordEntityTarget(targets *map[entityTargetKey]entityTarget, resource pcommon.Resource, targetLabels []prompb.Label, ts pcommon.Timestamp) {
	var job, instance string
	for _, l := range targetLabels {
		switch l.Name {
		case model.JobLabel:
			job = l.Value
		case model.InstanceLabel:
			instance = l.Value
		}
	}
	if *targets == nil {
		*targets = make(map[entityTargetKey]entityTarget)
	}
	key := entityTargetKey{job: job, instance: instance}
	if existing, ok := (*targets)[key]; ok && ts < existing.timestamp {
		return
	}
	(*targets)[key] = entityTarget{
		resource:  resource,
		job:       job,
		instance:  instance,
		timestamp: ts,
	}
}

func sortedEntityTargetKeys(targets map[entityTargetKey]entityTarget) []entityTargetKey {
	keys := make([]entityTargetKey, 0, len(targets))
	for k := range targets {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b entityTargetKey) int {
		if c := strings.Compare(a.job, b.job); c != 0 {
			return c
		}
		return strings.Compare(a.instance, b.instance)
	})
	return keys
}

func buildEntityInfoLabelSets(target entityTarget, labelNamer otlptranslator.LabelNamer) ([][]prompb.Label, error) {
	infos, err := prometheustranslator.BuildEntityInfos(target.resource, labelNamer.Build)
	if err != nil || len(infos) == 0 {
		return nil, err
	}

	out := make([][]prompb.Label, 0, len(infos))
	var errs error
	for _, info := range infos {
		encodedIDKeys, encErr := prometheustranslator.EncodeEntityKeys(info.IDKeys)
		if encErr != nil {
			errs = multierr.Append(errs, encErr)
			continue
		}

		lbls := make([]prompb.Label, 0, 7)
		lbls = append(lbls, prompb.Label{Name: model.MetricNameLabel, Value: prometheustranslator.EntityInfoMetricName})
		if target.job != "" {
			lbls = append(lbls, prompb.Label{Name: model.JobLabel, Value: target.job})
		}
		if target.instance != "" {
			lbls = append(lbls, prompb.Label{Name: model.InstanceLabel, Value: target.instance})
		}
		lbls = append(lbls, prompb.Label{Name: prometheustranslator.EntityTypeLabelKey, Value: info.Type})
		if info.SchemaURL != "" {
			lbls = append(lbls, prompb.Label{Name: prometheustranslator.EntitySchemaURLLabelKey, Value: info.SchemaURL})
		}
		lbls = append(lbls, prompb.Label{Name: prometheustranslator.EntityIDKeysLabelKey, Value: encodedIDKeys})
		if len(info.DescriptionKeys) > 0 {
			encodedDescKeys, encErr := prometheustranslator.EncodeEntityKeys(info.DescriptionKeys)
			if encErr != nil {
				errs = multierr.Append(errs, encErr)
				continue
			}
			lbls = append(lbls, prompb.Label{Name: prometheustranslator.EntityDescriptionKeysLabelKey, Value: encodedDescKeys})
		}
		out = append(out, lbls)
	}
	return out, errs
}

func (c *prometheusConverter) addResourceEntityInfos() error {
	if len(c.entityTargets) == 0 {
		return nil
	}
	var errs error
	for _, key := range sortedEntityTargetKeys(c.entityTargets) {
		target := c.entityTargets[key]
		labelSets, err := buildEntityInfoLabelSets(target, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		ts := convertTimeStamp(target.timestamp)
		for _, lbls := range labelSets {
			c.addSample(&prompb.Sample{
				Value:     1,
				Timestamp: ts,
			}, lbls)
		}
	}
	return errs
}

func (c *prometheusConverterV2) addResourceEntityInfosV2() error {
	if len(c.entityTargets) == 0 {
		return nil
	}
	var errs error
	md := metadata{
		Type: writev2.Metadata_METRIC_TYPE_INFO,
		Help: "Entity metadata",
	}
	for _, key := range sortedEntityTargetKeys(c.entityTargets) {
		target := c.entityTargets[key]
		labelSets, err := buildEntityInfoLabelSets(target, c.labelNamer)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
		ts := convertTimeStamp(target.timestamp)
		for _, lbls := range labelSets {
			c.addSample(&writev2.Sample{
				Value:     1,
				Timestamp: ts,
			}, lbls, md)
		}
	}
	return errs
}
