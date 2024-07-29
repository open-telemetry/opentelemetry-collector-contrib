package helm

import (
	_ "embed"
	"fmt"

	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/DataDog/test-infra-definitions/common/config"
	"github.com/DataDog/test-infra-definitions/components"
	"github.com/DataDog/test-infra-definitions/components/datadog/agent"
	"github.com/DataDog/test-infra-definitions/resources/helm"
)

//go:embed values.yaml
var values string

type OtelCollector struct {
	pulumi.ResourceState
	components.Component
}

func NewOtelCollector(e config.Env, resourceName string, kubeProvider *kubernetes.Provider) (*OtelCollector, error) {
	secret, err := agent.NewImagePullSecret(e, "default", pulumi.Provider(kubeProvider))
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

	valuesYAML = append(valuesYAML, pulumi.NewStringAsset(values), imagePullSecretValue)
	return components.NewComponent(e, resourceName, func(comp *OtelCollector) error {

		_, err := helm.NewInstallation(e, helm.InstallArgs{
			RepoURL:     "https://open-telemetry.github.io/opentelemetry-helm-charts",
			ChartName:   "opentelemetry-collector",
			Namespace:   "default",
			InstallName: "otel-collector",
			ValuesYAML:  valuesYAML,
		}, pulumi.Provider(kubeProvider), pulumi.Parent(comp), pulumi.DependsOn([]pulumi.Resource{secret}))

		return err
	})
}
