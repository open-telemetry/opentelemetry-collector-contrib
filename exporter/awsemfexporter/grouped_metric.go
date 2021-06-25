// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"encoding/json"
	"log"

	"go.opentelemetry.io/collector/model/pdata"
	"go.uber.org/zap"

	aws "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics"
)

// groupedMetric defines set of metrics with same namespace, timestamp and labels
type groupedMetric struct {
	labels   map[string]string
	metrics  map[string]*metricInfo
	metadata cWMetricMetadata
}

// metricInfo defines value and unit for OT Metrics
type metricInfo struct {
	value interface{}
	unit  string
}

// addToGroupedMetric processes OT metrics and adds them into GroupedMetric buckets
func addToGroupedMetric(pmd *pdata.Metric, groupedMetrics map[interface{}]*GroupedMetric, metadata CWMetricMetadata, logger *zap.Logger, descriptor map[string]MetricDescriptor, config *Config) {
	if pmd == nil {
		return
	}

	metricName := pmd.Name()
	dps := getDataPoints(pmd, metadata, logger)
	if dps == nil || dps.Len() == 0 {
		return
	}

	for i := 0; i < dps.Len(); i++ {
		dp, retained := dps.At(i)
		if !retained {
			continue
		}

		labels := dp.Labels

		if isPod, ok := labels["Type"]; ok {
			if isPod == "Pod" && config.CreateHighLevelObject {
				addKubernetesWrapper(labels)
			} else if isPod == "Container" && config.CreateHighLevelObject {
				addKubernetesWrapper(labels)
			}
		}

		metric := &MetricInfo{
			Value: dp.Value,
			Unit:  translateUnit(pmd, descriptor),
		}

		if dp.timestampMs > 0 {
			metadata.timestampMs = dp.timestampMs
		}

		// Extra params to use when grouping metrics
		groupKey := groupedMetricKey(metadata.groupedMetricMetadata, labels)
		if _, ok := groupedMetrics[groupKey]; ok {
			// if metricName already exists in metrics map, print warning log
			if _, ok := groupedMetrics[groupKey].metrics[metricName]; ok {
				logger.Warn(
					"Duplicate metric found",
					zap.String("Name", metricName),
					zap.Any("Labels", labels),
				)
			} else {
				groupedMetrics[groupKey].metrics[metricName] = metric
			}
		} else {
			groupedMetrics[groupKey] = &groupedMetric{
				labels:   labels,
				metrics:  map[string]*metricInfo{(metricName): metric},
				metadata: metadata,
			}
		}
	}
}

type kubernetesObj struct {
	Host           string               `json:"`
	Labels         internalLabelsObj    `json:`
	Namespace_name string               `json:`
	Pod_id         string               `json:`
	Pod_name       string               `json:`
	Pod_owners     internalPodOwnersObj `json:`
	Service_name   string               `json:`
}

type internalLabelsObj struct {
	App               string `json:`
	Pod_template_hash string `json:`
}

type internalPodOwnersObj struct {
	Owner_kind string `json:`
	Owner_name string `json:`
}

func addKubernetesWrapper(labels map[string]string) {
	//create schema
	schema := kubernetesObj{}
	schema.Host = "host_name"
	schema.Labels =
		internalLabelsObj{
			App:               "app",
			Pod_template_hash: "pod_template_hash",
		}
	schema.Namespace_name = "namespace_name"
	schema.Pod_id = "pod_id"
	schema.Pod_name = "pod_name"
	schema.Pod_owners =
		internalPodOwnersObj{
			Owner_kind: "owner_kind",
			Owner_name: "owner_name",
		}
	schema.Service_name = "service_name"

	labels["kubernetes"] = recursivelyFillInStruct(labels, schema)
}

func recursivelyFillInStruct(labels map[string]string, schema interface{}) string {
	jsonBytes, err := json.Marshal(schema)
	if err != nil {
		log.Fatal(err)
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(jsonBytes, &m)
	if err != nil {
		log.Fatal(err)
	}

	m = recursivelyFillInMap(labels, m)
	jsonBytes, err = json.Marshal(m)
	if err != nil {
		log.Fatal(err)
	}

	jsonString := string(jsonBytes)
	return jsonString

}

func recursivelyFillInMap(labels map[string]string, schema map[string]interface{}) map[string]interface{} {
	//Iterate over the keys of the schema
	for k, v := range schema {
		//Check if it is nested or not
		nestedObj, isNested := v.(map[string]interface{})
		if isNested {
			//recursively fill in the nested object
			schema[k] = recursivelyFillInMap(labels, nestedObj)
			//if the object is empty delete it
			mapForm, _ := schema[k].(map[string]interface{})
			if len(mapForm) == 0 {
				delete(schema, k)
			}
		} else {
			stringVal, isString := v.(string)
			if !isString {
				log.Fatal("Non string, struct value found in schema")
			}
			labelVal, exists := labels[stringVal]
			if !exists {
				delete(schema, k)
			} else {
				schema[k] = labelVal
			}
		}

	}
	return schema
}

func groupedMetricKey(metadata GroupedMetricMetadata, labels map[string]string) aws.Key {
	return aws.NewKey(metadata, labels)
}

func translateUnit(metric *pdata.Metric, descriptor map[string]MetricDescriptor) string {
	unit := metric.Unit()
	if descriptor, exists := descriptor[metric.Name()]; exists {
		if unit == "" || descriptor.overwrite {
			return descriptor.unit
		}
	}
	switch unit {
	case "ms":
		unit = "Milliseconds"
	case "s":
		unit = "Seconds"
	case "us":
		unit = "Microseconds"
	case "By":
		unit = "Bytes"
	case "Bi":
		unit = "Bits"
	}
	return unit
}
