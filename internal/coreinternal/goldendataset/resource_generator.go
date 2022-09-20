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
	attrMap.PutString(conventions.AttributeServiceName, "customers")
	attrMap.PutString(conventions.AttributeServiceNamespace, "production")
	attrMap.PutString(conventions.AttributeServiceVersion, "semver:0.7.3")
	subMap := attrMap.PutEmptyMap(conventions.AttributeHostName)
	subMap.PutString("public", "tc-prod9.internal.example.com")
	subMap.PutString("internal", "172.18.36.18")
	attrMap.PutString(conventions.AttributeHostImageID, "661ADFA6-E293-4870-9EFA-1AA052C49F18")
	attrMap.PutString(conventions.AttributeTelemetrySDKLanguage, conventions.AttributeTelemetrySDKLanguageJava)
	attrMap.PutString(conventions.AttributeTelemetrySDKName, "opentelemetry")
	attrMap.PutString(conventions.AttributeTelemetrySDKVersion, "0.3.0")
}

func appendCloudVMAttributes(attrMap pcommon.Map) {
	attrMap.PutString(conventions.AttributeServiceName, "shoppingcart")
	attrMap.PutString(conventions.AttributeServiceName, "customers")
	attrMap.PutString(conventions.AttributeServiceNamespace, "production")
	attrMap.PutString(conventions.AttributeServiceVersion, "semver:0.7.3")
	attrMap.PutString(conventions.AttributeTelemetrySDKLanguage, conventions.AttributeTelemetrySDKLanguageJava)
	attrMap.PutString(conventions.AttributeTelemetrySDKName, "opentelemetry")
	attrMap.PutString(conventions.AttributeTelemetrySDKVersion, "0.3.0")
	attrMap.PutString(conventions.AttributeHostID, "57e8add1f79a454bae9fb1f7756a009a")
	attrMap.PutString(conventions.AttributeHostName, "env-check")
	attrMap.PutString(conventions.AttributeHostImageID, "5.3.0-1020-azure")
	attrMap.PutString(conventions.AttributeHostType, "B1ms")
	attrMap.PutString(conventions.AttributeCloudProvider, "azure")
	attrMap.PutString(conventions.AttributeCloudAccountID, "2f5b8278-4b80-4930-a6bb-d86fc63a2534")
	attrMap.PutString(conventions.AttributeCloudRegion, "South Central US")
}

func appendOnpremK8sAttributes(attrMap pcommon.Map) {
	attrMap.PutString(conventions.AttributeContainerName, "cert-manager")
	attrMap.PutString(conventions.AttributeContainerImageName, "quay.io/jetstack/cert-manager-controller")
	attrMap.PutString(conventions.AttributeContainerImageTag, "v0.14.2")
	attrMap.PutString(conventions.AttributeK8SClusterName, "docker-desktop")
	attrMap.PutString(conventions.AttributeK8SNamespaceName, "cert-manager")
	attrMap.PutString(conventions.AttributeK8SDeploymentName, "cm-1-cert-manager")
	attrMap.PutString(conventions.AttributeK8SPodName, "cm-1-cert-manager-6448b4949b-t2jtd")
	attrMap.PutString(conventions.AttributeHostName, "docker-desktop")
}

func appendCloudK8sAttributes(attrMap pcommon.Map) {
	attrMap.PutString(conventions.AttributeContainerName, "otel-collector")
	attrMap.PutString(conventions.AttributeContainerImageName, "otel/opentelemetry-collector-contrib")
	attrMap.PutString(conventions.AttributeContainerImageTag, "0.4.0")
	attrMap.PutString(conventions.AttributeK8SClusterName, "erp-dev")
	attrMap.PutString(conventions.AttributeK8SNamespaceName, "monitoring")
	attrMap.PutString(conventions.AttributeK8SDeploymentName, "otel-collector")
	attrMap.PutString(conventions.AttributeK8SDeploymentUID, "4D614B27-EDAF-409B-B631-6963D8F6FCD4")
	attrMap.PutString(conventions.AttributeK8SReplicaSetName, "otel-collector-2983fd34")
	attrMap.PutString(conventions.AttributeK8SReplicaSetUID, "EC7D59EF-D5B6-48B7-881E-DA6B7DD539B6")
	attrMap.PutString(conventions.AttributeK8SPodName, "otel-collector-6484db5844-c6f9m")
	attrMap.PutString(conventions.AttributeK8SPodUID, "FDFD941E-2A7A-4945-B601-88DD486161A4")
	attrMap.PutString(conventions.AttributeHostID, "ec2e3fdaffa294348bdf355156b94cda")
	attrMap.PutString(conventions.AttributeHostName, "10.99.118.157")
	attrMap.PutString(conventions.AttributeHostImageID, "ami-011c865bf7da41a9d")
	attrMap.PutString(conventions.AttributeHostType, "m5.xlarge")
	attrMap.PutString(conventions.AttributeCloudProvider, "aws")
	attrMap.PutString(conventions.AttributeCloudAccountID, "12345678901")
	attrMap.PutString(conventions.AttributeCloudRegion, "us-east-1")
	attrMap.PutString(conventions.AttributeCloudAvailabilityZone, "us-east-1c")
}

func appendFassAttributes(attrMap pcommon.Map) {
	attrMap.PutString(conventions.AttributeFaaSID, "https://us-central1-dist-system-demo.cloudfunctions.net/env-vars-print")
	attrMap.PutString(conventions.AttributeFaaSName, "env-vars-print")
	attrMap.PutString(conventions.AttributeFaaSVersion, "semver:1.0.0")
	attrMap.PutString(conventions.AttributeCloudProvider, "gcp")
	attrMap.PutString(conventions.AttributeCloudAccountID, "opentelemetry")
	attrMap.PutString(conventions.AttributeCloudRegion, "us-central1")
	attrMap.PutString(conventions.AttributeCloudAvailabilityZone, "us-central1-a")
}

func appendExecAttributes(attrMap pcommon.Map) {
	attrMap.PutString(conventions.AttributeProcessExecutableName, "otelcol")
	parts := attrMap.PutEmptySlice(conventions.AttributeProcessCommandLine)
	parts.AppendEmpty().SetStringVal("otelcol")
	parts.AppendEmpty().SetStringVal("--config=/etc/otel-collector-config.yaml")
	attrMap.PutString(conventions.AttributeProcessExecutablePath, "/usr/local/bin/otelcol")
	attrMap.PutInt(conventions.AttributeProcessPID, 2020)
	attrMap.PutString(conventions.AttributeProcessOwner, "otel")
	attrMap.PutString(conventions.AttributeOSType, "linux")
	attrMap.PutString(conventions.AttributeOSDescription,
		"Linux ubuntu 5.4.0-42-generic #46-Ubuntu SMP Fri Jul 10 00:24:02 UTC 2020 x86_64 x86_64 x86_64 GNU/Linux")
}
