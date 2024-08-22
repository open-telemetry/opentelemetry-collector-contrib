// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package helm

import (
	_ "embed"
	"fmt"

	"github.com/DataDog/test-infra-definitions/common/config"
	"github.com/DataDog/test-infra-definitions/components"
	"github.com/DataDog/test-infra-definitions/components/datadog/agent"
	"github.com/DataDog/test-infra-definitions/components/datadog/fakeintake"
	"github.com/DataDog/test-infra-definitions/resources/helm"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	otelcomp "github.com/DataDog/opentelemetry-collector-contrib/e2e-tests/otel-collector/component"
	"github.com/DataDog/opentelemetry-collector-contrib/e2e-tests/otel-collector/otelparams"
)

//go:embed values.yaml
var values string

func NewOTelCollector(e config.Env, resourceName string, opts ...otelparams.Option) (*otelcomp.OTelCollector, error) {
	params, err := otelparams.NewParams(opts...)
	if err != nil {
		return nil, err
	}

	secret, err := agent.NewImagePullSecret(e, "default", params.PulumiResourceOptions...)
	if err != nil {
		return nil, err
	}
	imagePullSecretValue := secret.Metadata.Name().Elem().ApplyT(func(name string) (pulumi.Asset, error) {
		yamlValues := fmt.Sprintf(`
imagePullSecrets:
  - name: %s
`, name)
		return pulumi.NewStringAsset(yamlValues), nil
	}).(pulumi.AssetOutput)
	valuesYAML := pulumi.AssetOrArchiveArray{}

	if params.Fakeintake != nil {
		valuesYAML = append(valuesYAML, buildFakeintakeValues(params.Fakeintake))
	}
	valuesYAML = append(valuesYAML, pulumi.NewStringAsset(values), imagePullSecretValue)
	valuesYAML = append(valuesYAML, params.HelmValues...)
	pulumiResourceOpts := append(params.PulumiResourceOptions, pulumi.DependsOn([]pulumi.Resource{secret}))

	return components.NewComponent(e, resourceName, func(comp *otelcomp.OTelCollector) error {

		release, err := helm.NewInstallation(e, helm.InstallArgs{
			RepoURL:     "https://open-telemetry.github.io/opentelemetry-helm-charts",
			ChartName:   "opentelemetry-collector",
			Namespace:   "default",
			InstallName: "otel-collector",
			ValuesYAML:  valuesYAML,
		}, pulumiResourceOpts...)

		comp.LabelSelectors = pulumi.Map{
			"app.kubernetes.io/name": release.Name,
		}

		return err
	})
}

func buildFakeintakeValues(fakeintake *fakeintake.Fakeintake) pulumi.AssetOutput {
	return fakeintake.URL.ApplyT(func(url string) (pulumi.Asset, error) {
		defaultValuesYAML := fmt.Sprintf(`
config:
  exporters:
    datadog:
      metrics:
        endpoint: %[1]s
      traces:
        endpoint: %[1]s
      logs:
        endpoint: %[1]s
`, url)

		return pulumi.NewStringAsset(string(defaultValuesYAML)), nil

	}).(pulumi.AssetOutput)
}
