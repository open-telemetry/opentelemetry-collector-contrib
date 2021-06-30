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

	"encoding/json"
	"errors"

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
func addToGroupedMetric(pmd *pdata.Metric, groupedMetrics map[interface{}]*GroupedMetric, metadata CWMetricMetadata, logger *zap.Logger, descriptor map[string]MetricDescriptor, config *Config) error {
	if pmd == nil {
		return nil
	}

	metricName := pmd.Name()
	dps := getDataPoints(pmd, metadata, logger)
	if dps == nil || dps.Len() == 0 {
		return nil
	}

	for i := 0; i < dps.Len(); i++ {
		dp, retained := dps.At(i)
		if !retained {
			continue
		}

		labels := dp.Labels

		if metricType, ok := labels["Type"]; ok {
			if(metricType == "Pod" && config.CreateEKSFargateKubernetesObject){
				err := addKubernetesWrapper(labels)
				if err != nil {
					logger.Warn("Issue forming Kubernetes Object", zap.Error(err))
					return err
				}
			} else if (metricType == "Container" && config.CreateEKSFargateKubernetesObject){
				err  := addKubernetesWrapper(labels)
				if err != nil {
					logger.Warn("Issue forming Kubernetes Object", zap.Error(err))
					return err
				}
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

	return nil
}

type kubernetesObj struct{
	Container_name string `json:`
	Docker internalDockerObj `json:`
	Host string `json:`
	Labels internalLabelsObj `json:`
	Namespace_name string `json:`
	Pod_id string `json:`
	Pod_name string `json:`
	Pod_owners internalPodOwnersObj `json:`
	Service_name string `json:`
}

type internalDockerObj struct{
	Container_id string `json:`
}

type internalLabelsObj struct{
	App string `json:`
	Pod_template_hash string `json: "pod-template-hash"`
}

type internalPodOwnersObj struct {
	Owner_kind string `json:`
	Owner_name string `json:`
}

func addKubernetesWrapper(labels map[string]string) error {
	//create schema
	schema := kubernetesObj{}
	schema.Container_name = "container_name"
	schema.Docker =
	internalDockerObj{
		Container_id: "container_id"
	}
	schema.Host = "host_name"
	schema.Labels =
	internalLabelsObj{
		App: "app",
		Pod_template_hash: "pod-template-hash",
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

	var err error
	labels["kubernetes"], err = recursivelyFillInStruct(labels, schema)
	if err != nil {
		return err
	}
	return nil
}

func recursivelyFillInStruct(labels map[string]string, schema interface{}) (string, error) {
	jsonBytes, err := json.Marshal(schema)
	if err != nil {
		return "", err
	}

	m := make(map[string]interface{})
	err = json.Unmarshal(jsonBytes, &m)
	if err != nil {
		return "", err
	}

	m, err = recursivelyFillInMap(labels, m)
	if err != nil {
		return "", err
	}
	jsonBytes, err = json.Marshal(m)
	if err != nil {
		return "", err
	}

	jsonString := string(jsonBytes)
	return jsonString, nil

}

func recursivelyFillInMap(labels map[string]string, schema map[string]interface{}) (map[string]interface{}, error) {
	//Iterate over the keys of the schema
	var err error
	for k,v := range schema{
		//Check if it is nested or not
		nestedObj, isNested := v.(map[string]interface{})
		if isNested {
			//recursively fill in the nested object
			schema[k], err = recursivelyFillInMap(labels, nestedObj)
			if err != nil {
				return nil, err
			}
			//if the object is empty delete it
			mapForm, _ := schema[k].(map[string]interface{})
			if len(mapForm) == 0 {
				delete(schema, k)
			}
		} else {
			stringVal, isString := v.(string)
			if !isString {
				return nil, errors.New("Non string, struct value found in schema")
			}
			labelVal, exists := labels[stringVal]
			if !exists {
				delete(schema, k)
			} else {
				schema[k] = labelVal
			}
		}

	}

	return schema, nil
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
