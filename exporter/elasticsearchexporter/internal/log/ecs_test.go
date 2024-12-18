// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	semconv "go.opentelemetry.io/collector/semconv/v1.22.0"
)

func TestEncodeLogECSModeDuplication(t *testing.T) {
	resource := pcommon.NewResource()
	err := resource.Attributes().FromRaw(map[string]any{
		semconv.AttributeServiceName:    "foo.bar",
		semconv.AttributeHostName:       "localhost",
		semconv.AttributeServiceVersion: "1.1.0",
		semconv.AttributeOSType:         "darwin",
		semconv.AttributeOSDescription:  "Mac OS Mojave",
		semconv.AttributeOSName:         "Mac OS X",
		semconv.AttributeOSVersion:      "10.14.1",
	})
	require.NoError(t, err)

	want := `{"@timestamp":"2024-03-12T20:00:41.123456789Z","agent":{"name":"otlp"},"container":{"image":{"tag":["v3.4.0"]}},"event":{"action":"user-password-change"},"host":{"hostname":"localhost","name":"localhost","os":{"full":"Mac OS Mojave","name":"Mac OS X","platform":"darwin","type":"macos","version":"10.14.1"}},"service":{"name":"foo.bar","version":"1.1.0"}}`
	require.NoError(t, err)

	resourceContainerImageTags := resource.Attributes().PutEmptySlice(semconv.AttributeContainerImageTags)
	err = resourceContainerImageTags.FromRaw([]any{"v3.4.0"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	record := plog.NewLogRecord()
	err = record.Attributes().FromRaw(map[string]any{
		"event.name": "user-password-change",
	})
	require.NoError(t, err)
	observedTimestamp := pcommon.Timestamp(1710273641123456789)
	record.SetObservedTimestamp(observedTimestamp)

	e := ecsEncoder{
		dedot: true,
	}
	doc, err := e.EncodeLog(resource, "", record, scope, "")
	require.NoError(t, err)

	assert.Equal(t, want, string(doc))
}

func TestEncodeLogECSMode(t *testing.T) {
	resource := pcommon.NewResource()
	err := resource.Attributes().FromRaw(map[string]any{
		semconv.AttributeServiceName:           "foo.bar",
		semconv.AttributeServiceVersion:        "1.1.0",
		semconv.AttributeServiceInstanceID:     "i-103de39e0a",
		semconv.AttributeTelemetrySDKName:      "opentelemetry",
		semconv.AttributeTelemetrySDKVersion:   "7.9.12",
		semconv.AttributeTelemetrySDKLanguage:  "perl",
		semconv.AttributeCloudProvider:         "gcp",
		semconv.AttributeCloudAccountID:        "19347013",
		semconv.AttributeCloudRegion:           "us-west-1",
		semconv.AttributeCloudAvailabilityZone: "us-west-1b",
		semconv.AttributeCloudPlatform:         "gke",
		semconv.AttributeContainerName:         "happy-seger",
		semconv.AttributeContainerID:           "e69cc5d3dda",
		semconv.AttributeContainerImageName:    "my-app",
		semconv.AttributeContainerRuntime:      "docker",
		semconv.AttributeHostName:              "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		semconv.AttributeHostID:                "i-103de39e0a",
		semconv.AttributeHostType:              "t2.medium",
		semconv.AttributeHostArch:              "x86_64",
		semconv.AttributeProcessPID:            9833,
		semconv.AttributeProcessCommandLine:    "/usr/bin/ssh -l user 10.0.0.16",
		semconv.AttributeProcessExecutablePath: "/usr/bin/ssh",
		semconv.AttributeProcessRuntimeName:    "OpenJDK Runtime Environment",
		semconv.AttributeProcessRuntimeVersion: "14.0.2",
		semconv.AttributeOSType:                "darwin",
		semconv.AttributeOSDescription:         "Mac OS Mojave",
		semconv.AttributeOSName:                "Mac OS X",
		semconv.AttributeOSVersion:             "10.14.1",
		semconv.AttributeDeviceID:              "00000000-54b3-e7c7-0000-000046bffd97",
		semconv.AttributeDeviceModelIdentifier: "SM-G920F",
		semconv.AttributeDeviceModelName:       "Samsung Galaxy S6",
		semconv.AttributeDeviceManufacturer:    "Samsung",
		"k8s.namespace.name":                   "default",
		"k8s.node.name":                        "node-1",
		"k8s.pod.name":                         "opentelemetry-pod-autoconf",
		"k8s.pod.uid":                          "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff",
		"k8s.deployment.name":                  "coredns",
		semconv.AttributeK8SJobName:            "job.name",
		semconv.AttributeK8SCronJobName:        "cronjob.name",
		semconv.AttributeK8SStatefulSetName:    "statefulset.name",
		semconv.AttributeK8SReplicaSetName:     "replicaset.name",
		semconv.AttributeK8SDaemonSetName:      "daemonset.name",
		semconv.AttributeK8SContainerName:      "container.name",
		semconv.AttributeK8SClusterName:        "cluster.name",
	})
	require.NoError(t, err)

	resourceContainerImageTags := resource.Attributes().PutEmptySlice(semconv.AttributeContainerImageTags)
	err = resourceContainerImageTags.FromRaw([]any{"v3.4.0"})
	require.NoError(t, err)

	scope := pcommon.NewInstrumentationScope()

	record := plog.NewLogRecord()
	err = record.Attributes().FromRaw(map[string]any{
		"event.name": "user-password-change",
	})
	require.NoError(t, err)
	observedTimestamp := pcommon.Timestamp(1710273641123456789)
	record.SetObservedTimestamp(observedTimestamp)

	e := ecsEncoder{}
	data, err := e.EncodeLog(resource, "", record, scope, "")
	require.NoError(t, err)

	require.JSONEq(t, `{
		"@timestamp":                 "2024-03-12T20:00:41.123456789Z",
		"service.name":               "foo.bar",
		"service.version":            "1.1.0",
		"service.node.name":          "i-103de39e0a",
		"agent.name":                 "opentelemetry/perl",
		"agent.version":              "7.9.12",
		"cloud.provider":             "gcp",
		"cloud.account.id":           "19347013",
		"cloud.region":               "us-west-1",
		"cloud.availability_zone":    "us-west-1b",
		"cloud.service.name":         "gke",
		"container.name":             "happy-seger",
		"container.id":               "e69cc5d3dda",
		"container.image.name":       "my-app",
		"container.image.tag":        ["v3.4.0"],
		"container.runtime":          "docker",
		"host.hostname":              "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		"host.name":                  "i-103de39e0a.gke.us-west-1b.cloud.google.com",
		"host.id":                    "i-103de39e0a",
		"host.type":                  "t2.medium",
		"host.architecture":          "x86_64",
		"process.pid":                9833,
		"process.command_line":       "/usr/bin/ssh -l user 10.0.0.16",
		"process.executable":         "/usr/bin/ssh",
		"service.runtime.name":       "OpenJDK Runtime Environment",
		"service.runtime.version":    "14.0.2",
		"host.os.platform":           "darwin",
		"host.os.full":               "Mac OS Mojave",
		"host.os.name":               "Mac OS X",
		"host.os.version":            "10.14.1",
		"host.os.type":               "macos",
		"device.id":                  "00000000-54b3-e7c7-0000-000046bffd97",
		"device.model.identifier":    "SM-G920F",
		"device.model.name":          "Samsung Galaxy S6",
		"device.manufacturer":        "Samsung",
		"event.action":               "user-password-change",
		"kubernetes.namespace":       "default",
		"kubernetes.node.name":       "node-1",
		"kubernetes.pod.name":        "opentelemetry-pod-autoconf",
		"kubernetes.pod.uid":         "275ecb36-5aa8-4c2a-9c47-d8bb681b9aff",
		"kubernetes.deployment.name": "coredns",
		"kubernetes.job.name":         "job.name",
		"kubernetes.cronjob.name":     "cronjob.name",
		"kubernetes.statefulset.name": "statefulset.name",
		"kubernetes.replicaset.name":  "replicaset.name",
		"kubernetes.daemonset.name":   "daemonset.name",
		"kubernetes.container.name":   "container.name",
		"orchestrator.cluster.name":   "cluster.name"
	}`, string(data))
}

func TestEncodeLogECSModeAgentName(t *testing.T) {
	tests := map[string]struct {
		telemetrySdkName     string
		telemetrySdkLanguage string
		telemetryDistroName  string

		expectedAgentName           string
		expectedServiceLanguageName string
	}{
		"none_set": {
			expectedAgentName:           "otlp",
			expectedServiceLanguageName: "unknown",
		},
		"name_set": {
			telemetrySdkName:            "opentelemetry",
			expectedAgentName:           "opentelemetry",
			expectedServiceLanguageName: "unknown",
		},
		"language_set": {
			telemetrySdkLanguage:        "java",
			expectedAgentName:           "otlp/java",
			expectedServiceLanguageName: "java",
		},
		"distro_set": {
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "otlp/unknown/parts-unlimited-java",
			expectedServiceLanguageName: "unknown",
		},
		"name_language_set": {
			telemetrySdkName:            "opentelemetry",
			telemetrySdkLanguage:        "java",
			expectedAgentName:           "opentelemetry/java",
			expectedServiceLanguageName: "java",
		},
		"name_distro_set": {
			telemetrySdkName:            "opentelemetry",
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "opentelemetry/unknown/parts-unlimited-java",
			expectedServiceLanguageName: "unknown",
		},
		"language_distro_set": {
			telemetrySdkLanguage:        "java",
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "otlp/java/parts-unlimited-java",
			expectedServiceLanguageName: "java",
		},
		"name_language_distro_set": {
			telemetrySdkName:            "opentelemetry",
			telemetrySdkLanguage:        "java",
			telemetryDistroName:         "parts-unlimited-java",
			expectedAgentName:           "opentelemetry/java/parts-unlimited-java",
			expectedServiceLanguageName: "java",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := pcommon.NewResource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.telemetrySdkName != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetrySDKName, test.telemetrySdkName)
			}
			if test.telemetrySdkLanguage != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetrySDKLanguage, test.telemetrySdkLanguage)
			}
			if test.telemetryDistroName != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetryDistroName, test.telemetryDistroName)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)

			e := ecsEncoder{}
			data, err := e.EncodeLog(resource, "", record, scope, "")
			require.NoError(t, err)
			require.JSONEq(t, fmt.Sprintf(`{
				"@timestamp": "2024-03-13T23:50:59.123456789Z",
				"agent.name": %q
			}`, test.expectedAgentName), string(data))
		})
	}
}

func TestEncodeLogECSModeAgentVersion(t *testing.T) {
	tests := map[string]struct {
		telemetryDistroVersion string
		telemetrySdkVersion    string
		expectedAgentVersion   string
	}{
		"none_set": {
			expectedAgentVersion: "",
		},
		"distro_version_set": {
			telemetryDistroVersion: "7.9.2",
			expectedAgentVersion:   "7.9.2",
		},
		"sdk_version_set": {
			telemetrySdkVersion:  "8.10.3",
			expectedAgentVersion: "8.10.3",
		},
		"both_set": {
			telemetryDistroVersion: "7.9.2",
			telemetrySdkVersion:    "8.10.3",
			expectedAgentVersion:   "7.9.2",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := pcommon.NewResource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.telemetryDistroVersion != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetryDistroVersion, test.telemetryDistroVersion)
			}
			if test.telemetrySdkVersion != "" {
				resource.Attributes().PutStr(semconv.AttributeTelemetrySDKVersion, test.telemetrySdkVersion)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)

			e := ecsEncoder{}
			data, err := e.EncodeLog(resource, "", record, scope, "")
			require.NoError(t, err)

			if test.expectedAgentVersion == "" {
				require.JSONEq(t, `{
					"@timestamp": "2024-03-13T23:50:59.123456789Z",
					"agent.name": "otlp"
				}`, string(data))
			} else {
				require.JSONEq(t, fmt.Sprintf(`{
					"@timestamp": "2024-03-13T23:50:59.123456789Z",
					"agent.name": "otlp",
					"agent.version": %q
				}`, test.expectedAgentVersion), string(data))
			}
		})
	}
}

func TestEncodeLogECSModeHostOSType(t *testing.T) {
	tests := map[string]struct {
		osType string
		osName string

		expectedHostOsName     string
		expectedHostOsType     string
		expectedHostOsPlatform string
	}{
		"none_set": {
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "", // should not be set
			expectedHostOsPlatform: "", // should not be set
		},
		"type_windows": {
			osType:                 "windows",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "windows",
			expectedHostOsPlatform: "windows",
		},
		"type_linux": {
			osType:                 "linux",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "linux",
			expectedHostOsPlatform: "linux",
		},
		"type_darwin": {
			osType:                 "darwin",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "macos",
			expectedHostOsPlatform: "darwin",
		},
		"type_aix": {
			osType:                 "aix",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "aix",
		},
		"type_hpux": {
			osType:                 "hpux",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "hpux",
		},
		"type_solaris": {
			osType:                 "solaris",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "unix",
			expectedHostOsPlatform: "solaris",
		},
		"type_unknown": {
			osType:                 "unknown",
			expectedHostOsName:     "", // should not be set
			expectedHostOsType:     "", // should not be set
			expectedHostOsPlatform: "unknown",
		},
		"name_android": {
			osName:                 "Android",
			expectedHostOsName:     "Android",
			expectedHostOsType:     "android",
			expectedHostOsPlatform: "", // should not be set
		},
		"name_ios": {
			osName:                 "iOS",
			expectedHostOsName:     "iOS",
			expectedHostOsType:     "ios",
			expectedHostOsPlatform: "", // should not be set
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := pcommon.NewResource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.osType != "" {
				resource.Attributes().PutStr(semconv.AttributeOSType, test.osType)
			}
			if test.osName != "" {
				resource.Attributes().PutStr(semconv.AttributeOSName, test.osName)
			}

			timestamp := pcommon.Timestamp(1710373859123456789)
			record.SetTimestamp(timestamp)

			e := ecsEncoder{}
			data, err := e.EncodeLog(resource, "", record, scope, "")
			require.NoError(t, err)

			expectedJSON := `{"@timestamp":"2024-03-13T23:50:59.123456789Z", "agent.name":"otlp"`
			if test.expectedHostOsName != "" {
				expectedJSON += `, "host.os.name":` + strconv.Quote(test.expectedHostOsName)
			}
			if test.expectedHostOsType != "" {
				expectedJSON += `, "host.os.type":` + strconv.Quote(test.expectedHostOsType)
			}
			if test.expectedHostOsPlatform != "" {
				expectedJSON += `, "host.os.platform":` + strconv.Quote(test.expectedHostOsPlatform)
			}
			expectedJSON += "}"
			require.JSONEq(t, expectedJSON, string(data))
		})
	}
}

func TestEncodeLogECSModeTimestamps(t *testing.T) {
	tests := map[string]struct {
		timeUnixNano         int64
		observedTimeUnixNano int64
		expectedTimestamp    string
	}{
		"only_observed_set": {
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    "2024-03-12T20:00:41.123456789Z",
		},
		"both_set": {
			timeUnixNano:         1710273639345678901,
			observedTimeUnixNano: 1710273641123456789,
			expectedTimestamp:    "2024-03-12T20:00:39.345678901Z",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			resource := pcommon.NewResource()
			scope := pcommon.NewInstrumentationScope()
			record := plog.NewLogRecord()

			if test.timeUnixNano > 0 {
				record.SetTimestamp(pcommon.Timestamp(test.timeUnixNano))
			}
			if test.observedTimeUnixNano > 0 {
				record.SetObservedTimestamp(pcommon.Timestamp(test.observedTimeUnixNano))
			}

			e := ecsEncoder{}
			data, err := e.EncodeLog(resource, "", record, scope, "")
			require.NoError(t, err)

			require.JSONEq(t, fmt.Sprintf(
				`{"@timestamp":%q,"agent.name":"otlp"}`, test.expectedTimestamp,
			), string(data))
		})
	}
}
