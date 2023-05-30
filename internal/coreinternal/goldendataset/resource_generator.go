// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package goldendataset // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/goldendataset"

import (
	"go.opentelemetry.io/collector/pdata/pcommon"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
	attrMap.PutStr(conventions.AttributeServiceName, "customers")
	attrMap.PutStr(conventions.AttributeServiceNamespace, "production")
	attrMap.PutStr(conventions.AttributeServiceVersion, "semver:0.7.3")
	subMap := attrMap.PutEmptyMap(conventions.AttributeHostName)
	subMap.PutStr("public", "tc-prod9.internal.example.com")
	subMap.PutStr("internal", "172.18.36.18")
	attrMap.PutStr(conventions.AttributeHostImageID, "661ADFA6-E293-4870-9EFA-1AA052C49F18")
	attrMap.PutStr(conventions.AttributeTelemetrySDKLanguage, conventions.AttributeTelemetrySDKLanguageJava)
	attrMap.PutStr(conventions.AttributeTelemetrySDKName, "opentelemetry")
	attrMap.PutStr(conventions.AttributeTelemetrySDKVersion, "0.3.0")
}

func appendCloudVMAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeServiceName, "shoppingcart")
	attrMap.PutStr(conventions.AttributeServiceName, "customers")
	attrMap.PutStr(conventions.AttributeServiceNamespace, "production")
	attrMap.PutStr(conventions.AttributeServiceVersion, "semver:0.7.3")
	attrMap.PutStr(conventions.AttributeTelemetrySDKLanguage, conventions.AttributeTelemetrySDKLanguageJava)
	attrMap.PutStr(conventions.AttributeTelemetrySDKName, "opentelemetry")
	attrMap.PutStr(conventions.AttributeTelemetrySDKVersion, "0.3.0")
	attrMap.PutStr(conventions.AttributeHostID, "57e8add1f79a454bae9fb1f7756a009a")
	attrMap.PutStr(conventions.AttributeHostName, "env-check")
	attrMap.PutStr(conventions.AttributeHostImageID, "5.3.0-1020-azure")
	attrMap.PutStr(conventions.AttributeHostType, "B1ms")
	attrMap.PutStr(conventions.AttributeCloudProvider, "azure")
	attrMap.PutStr(conventions.AttributeCloudAccountID, "2f5b8278-4b80-4930-a6bb-d86fc63a2534")
	attrMap.PutStr(conventions.AttributeCloudRegion, "South Central US")
}

func appendOnpremK8sAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeContainerName, "cert-manager")
	attrMap.PutStr(conventions.AttributeContainerImageName, "quay.io/jetstack/cert-manager-controller")
	attrMap.PutStr(conventions.AttributeContainerImageTag, "v0.14.2")
	attrMap.PutStr(conventions.AttributeK8SClusterName, "docker-desktop")
	attrMap.PutStr(conventions.AttributeK8SNamespaceName, "cert-manager")
	attrMap.PutStr(conventions.AttributeK8SDeploymentName, "cm-1-cert-manager")
	attrMap.PutStr(conventions.AttributeK8SPodName, "cm-1-cert-manager-6448b4949b-t2jtd")
	attrMap.PutStr(conventions.AttributeHostName, "docker-desktop")
}

func appendCloudK8sAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeContainerName, "otel-collector")
	attrMap.PutStr(conventions.AttributeContainerImageName, "otel/opentelemetry-collector-contrib")
	attrMap.PutStr(conventions.AttributeContainerImageTag, "0.4.0")
	attrMap.PutStr(conventions.AttributeK8SClusterName, "erp-dev")
	attrMap.PutStr(conventions.AttributeK8SNamespaceName, "monitoring")
	attrMap.PutStr(conventions.AttributeK8SDeploymentName, "otel-collector")
	attrMap.PutStr(conventions.AttributeK8SDeploymentUID, "4D614B27-EDAF-409B-B631-6963D8F6FCD4")
	attrMap.PutStr(conventions.AttributeK8SReplicaSetName, "otel-collector-2983fd34")
	attrMap.PutStr(conventions.AttributeK8SReplicaSetUID, "EC7D59EF-D5B6-48B7-881E-DA6B7DD539B6")
	attrMap.PutStr(conventions.AttributeK8SPodName, "otel-collector-6484db5844-c6f9m")
	attrMap.PutStr(conventions.AttributeK8SPodUID, "FDFD941E-2A7A-4945-B601-88DD486161A4")
	attrMap.PutStr(conventions.AttributeHostID, "ec2e3fdaffa294348bdf355156b94cda")
	attrMap.PutStr(conventions.AttributeHostName, "10.99.118.157")
	attrMap.PutStr(conventions.AttributeHostImageID, "ami-011c865bf7da41a9d")
	attrMap.PutStr(conventions.AttributeHostType, "m5.xlarge")
	attrMap.PutStr(conventions.AttributeCloudProvider, "aws")
	attrMap.PutStr(conventions.AttributeCloudAccountID, "12345678901")
	attrMap.PutStr(conventions.AttributeCloudRegion, "us-east-1")
	attrMap.PutStr(conventions.AttributeCloudAvailabilityZone, "us-east-1c")
}

func appendFassAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeFaaSID, "https://us-central1-dist-system-demo.cloudfunctions.net/env-vars-print")
	attrMap.PutStr(conventions.AttributeFaaSName, "env-vars-print")
	attrMap.PutStr(conventions.AttributeFaaSVersion, "semver:1.0.0")
	attrMap.PutStr(conventions.AttributeCloudProvider, "gcp")
	attrMap.PutStr(conventions.AttributeCloudAccountID, "opentelemetry")
	attrMap.PutStr(conventions.AttributeCloudRegion, "us-central1")
	attrMap.PutStr(conventions.AttributeCloudAvailabilityZone, "us-central1-a")
}

func appendExecAttributes(attrMap pcommon.Map) {
	attrMap.PutStr(conventions.AttributeProcessExecutableName, "otelcol")
	parts := attrMap.PutEmptySlice(conventions.AttributeProcessCommandLine)
	parts.AppendEmpty().SetStr("otelcol")
	parts.AppendEmpty().SetStr("--config=/etc/otel-collector-config.yaml")
	attrMap.PutStr(conventions.AttributeProcessExecutablePath, "/usr/local/bin/otelcol")
	attrMap.PutInt(conventions.AttributeProcessPID, 2020)
	attrMap.PutStr(conventions.AttributeProcessOwner, "otel")
	attrMap.PutStr(conventions.AttributeOSType, "linux")
	attrMap.PutStr(conventions.AttributeOSDescription,
		"Linux ubuntu 5.4.0-42-generic #46-Ubuntu SMP Fri Jul 10 00:24:02 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux")
}
