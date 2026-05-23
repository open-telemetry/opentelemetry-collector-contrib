// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/otel/semconv/v1.18.0"
)

// GenerateResource generates a PData Resource object with representative attributes for the
// underlying resource type specified by the rscID input parameter.
func GenerateResource(rscID PICTInputResource) pcommon.Resource {
	resource := pcommon.NewResource()
	switch rscID {
	case ResourceEmpty:
		break
	case ResourceVMOnPrem:
		appendOnpremVMAttributes(resource.Attributes())
	case ResourceVMCloud:
		appendCloudVMAttributes(resource.Attributes())
	case ResourceK8sOnPrem:
		appendOnpremK8sAttributes(resource.Attributes())
	case ResourceK8sCloud:
		appendCloudK8sAttributes(resource.Attributes())
	case ResourceFaas:
		appendFassAttributes(resource.Attributes())
	case ResourceExec:
		appendExecAttributes(resource.Attributes())
	}
	return resource
}

func appendOnpremVMAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(string(conventions.ServiceNameKey), "customers")
	attrMap.PutStr(string(conventions.ServiceNamespaceKey), "production")
	attrMap.PutStr(string(conventions.ServiceVersionKey), "semver:0.7.3")
	subMap := attrMap.PutEmptyMap(string(conventions.HostNameKey))
	subMap.PutStr("public", "tc-prod9.internal.example.com")
	subMap.PutStr("internal", "172.18.36.18")
	attrMap.PutStr(string(conventions.HostImageIDKey), "661ADFA6-E293-4870-9EFA-1AA052C49F18")
	attrMap.PutStr(string(conventions.TelemetrySDKLanguageKey), conventions.TelemetrySDKLanguageJava.Value.AsString())
	attrMap.PutStr(string(conventions.TelemetrySDKNameKey), "opentelemetry")
	attrMap.PutStr(string(conventions.TelemetrySDKVersionKey), "0.3.0")
}

func appendCloudVMAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(string(conventions.ServiceNameKey), "shoppingcart")
	attrMap.PutStr(string(conventions.ServiceNameKey), "customers")
	attrMap.PutStr(string(conventions.ServiceNamespaceKey), "production")
	attrMap.PutStr(string(conventions.ServiceVersionKey), "semver:0.7.3")
	attrMap.PutStr(string(conventions.TelemetrySDKLanguageKey), conventions.TelemetrySDKLanguageJava.Value.AsString())
	attrMap.PutStr(string(conventions.TelemetrySDKNameKey), "opentelemetry")
	attrMap.PutStr(string(conventions.TelemetrySDKVersionKey), "0.3.0")
	attrMap.PutStr(string(conventions.HostIDKey), "57e8add1f79a454bae9fb1f7756a009a")
	attrMap.PutStr(string(conventions.HostNameKey), "env-check")
	attrMap.PutStr(string(conventions.HostImageIDKey), "5.3.0-1020-azure")
	attrMap.PutStr(string(conventions.HostTypeKey), "B1ms")
	attrMap.PutStr(string(conventions.CloudProviderKey), "azure")
	attrMap.PutStr(string(conventions.CloudAccountIDKey), "2f5b8278-4b80-4930-a6bb-d86fc63a2534")
	attrMap.PutStr(string(conventions.CloudRegionKey), "South Central US")
}

func appendOnpremK8sAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(string(conventions.ContainerNameKey), "cert-manager")
	attrMap.PutStr(string(conventions.ContainerImageNameKey), "quay.io/jetstack/cert-manager-controller")
	attrMap.PutStr(string(conventions.ContainerImageTagKey), "v0.14.2")
	attrMap.PutStr(string(conventions.K8SClusterNameKey), "docker-desktop")
	attrMap.PutStr(string(conventions.K8SNamespaceNameKey), "cert-manager")
	attrMap.PutStr(string(conventions.K8SDeploymentNameKey), "cm-1-cert-manager")
	attrMap.PutStr(string(conventions.K8SPodNameKey), "cm-1-cert-manager-6448b4949b-t2jtd")
	attrMap.PutStr(string(conventions.HostNameKey), "docker-desktop")
}

func appendCloudK8sAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(string(conventions.ContainerNameKey), "otel-collector")
	attrMap.PutStr(string(conventions.ContainerImageNameKey), "otel/opentelemetry-collector-contrib")
	attrMap.PutStr(string(conventions.ContainerImageTagKey), "0.4.0")
	attrMap.PutStr(string(conventions.K8SClusterNameKey), "erp-dev")
	attrMap.PutStr(string(conventions.K8SNamespaceNameKey), "monitoring")
	attrMap.PutStr(string(conventions.K8SDeploymentNameKey), "otel-collector")
	attrMap.PutStr(string(conventions.K8SDeploymentUIDKey), "4D614B27-EDAF-409B-B631-6963D8F6FCD4")
	attrMap.PutStr(string(conventions.K8SReplicaSetNameKey), "otel-collector-2983fd34")
	attrMap.PutStr(string(conventions.K8SReplicaSetUIDKey), "EC7D59EF-D5B6-48B7-881E-DA6B7DD539B6")
	attrMap.PutStr(string(conventions.K8SPodNameKey), "otel-collector-6484db5844-c6f9m")
	attrMap.PutStr(string(conventions.K8SPodUIDKey), "FDFD941E-2A7A-4945-B601-88DD486161A4")
	attrMap.PutStr(string(conventions.HostIDKey), "ec2e3fdaffa294348bdf355156b94cda")
	attrMap.PutStr(string(conventions.HostNameKey), "10.99.118.157")
	attrMap.PutStr(string(conventions.HostImageIDKey), "ami-011c865bf7da41a9d")
	attrMap.PutStr(string(conventions.HostTypeKey), "m5.xlarge")
	attrMap.PutStr(string(conventions.CloudProviderKey), "aws")
	attrMap.PutStr(string(conventions.CloudAccountIDKey), "12345678901")
	attrMap.PutStr(string(conventions.CloudRegionKey), "us-east-1")
	attrMap.PutStr(string(conventions.CloudAvailabilityZoneKey), "us-east-1c")
}

func appendFassAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(string(conventions.FaaSIDKey), "https://us-central1-dist-system-demo.cloudfunctions.net/env-vars-print")
	attrMap.PutStr(string(conventions.FaaSNameKey), "env-vars-print")
	attrMap.PutStr(string(conventions.FaaSVersionKey), "semver:1.0.0")
	attrMap.PutStr(string(conventions.CloudProviderKey), "gcp")
	attrMap.PutStr(string(conventions.CloudAccountIDKey), "opentelemetry")
	attrMap.PutStr(string(conventions.CloudRegionKey), "us-central1")
	attrMap.PutStr(string(conventions.CloudAvailabilityZoneKey), "us-central1-a")
}

func appendExecAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(string(conventions.ProcessExecutableNameKey), "otelcol")
	parts := attrMap.PutEmptySlice(string(conventions.ProcessCommandLineKey))
	parts.AppendEmpty().SetStr("otelcol")
	parts.AppendEmpty().SetStr("--config=/etc/otel-collector-config.yaml")
	attrMap.PutStr(string(conventions.ProcessExecutablePathKey), "/usr/local/bin/otelcol")
	attrMap.PutInt(string(conventions.ProcessPIDKey), 2020)
	attrMap.PutStr(string(conventions.ProcessOwnerKey), "otel")
	attrMap.PutStr(string(conventions.OSTypeKey), "linux")
	attrMap.PutStr(string(conventions.OSDescriptionKey),
		"Linux ubuntu 5.4.0-42-generic #46-Ubuntu SMP Fri Jul 10 00:24:02 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux")
}
