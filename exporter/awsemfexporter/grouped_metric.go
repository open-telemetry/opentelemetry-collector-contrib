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
			if (metricType == "Pod" || metricType == "Container") && config.CreateEKSFargateKubernetesObject {
				newObjectVal, err := applySchema(labels, config.EKSFargateKubernetesObjectSchema)
				if err != nil {
					logger.Warn("Issue forming Kubernetes Object", zap.Error(err))
					logger.Warn("Internal Schema", zap.String("internal schema", config.EKSFargateKubernetesObjectSchema))
					return err
				}

				labels["kubernetes"] = newObjectVal
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

type kubernetesObj struct {
	ContainerName string               `json: "container_name"`
	Docker        internalDockerObj    `json: "docker"`
	Host          string               `json: "host"`
	Labels        internalLabelsObj    `json: "labels"`
	NamespaceName string               `json: "namespace_name"`
	PodId         string               `json: "pod_id"`
	PodName       string               `json: "pod_name"`
	PodOwners     internalPodOwnersObj `json:"pod_owners"`
	ServiceName   string               `json: "service_name"`
}

type internalDockerObj struct {
	ContainerId string `json: "container_id"`
}

type internalLabelsObj struct {
	App             string `json: "app"`
	PodTemplateHash string `json: "pod-template-hash"`
}

type internalPodOwnersObj struct {
	OwnerKind string `json: "owner_kind"`
	OwnerName string `json: "owner_name"`
}

func addKubernetesWrapper(labels map[string]string) error {
	//create schema
	schema := kubernetesObj{}
	schema.ContainerName = "container_name"
	schema.Docker =
		internalDockerObj{
			ContainerId: "container_id",
		}
	schema.Host = "host_name"
	schema.Labels =
		internalLabelsObj{
			App:             "app",
			PodTemplateHash: "pod-template-hash",
		}
	schema.NamespaceName = "namespace_name"
	schema.PodId = "pod_id"
	schema.PodName = "pod_name"
	schema.PodOwners =
		internalPodOwnersObj{
			OwnerKind: "owner_kind",
			OwnerName: "owner_name",
		}
	schema.ServiceName = "service_name"

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

	return applySchema(labels, string(jsonBytes))

}

func applySchema(labels map[string]string, schema string) (string, error) {
	jsonBytes := []byte(schema)

	m := make(map[string]interface{})
	err := json.Unmarshal(jsonBytes, &m)
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
	for k, v := range schema {
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
			// This deleting implicitly deals with the difference between Container metrics and Pod metrics
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
