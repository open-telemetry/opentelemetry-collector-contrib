// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package otelcollector

import (
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/e2e"
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/runner"
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/utils/optional"
	"github.com/DataDog/test-infra-definitions/common/utils"
	fakeintakeComp "github.com/DataDog/test-infra-definitions/components/datadog/fakeintake"
	kubeComp "github.com/DataDog/test-infra-definitions/components/kubernetes"
	"github.com/DataDog/test-infra-definitions/resources/aws"
	"github.com/DataDog/test-infra-definitions/resources/local"
	"github.com/DataDog/test-infra-definitions/scenarios/aws/ec2"
	"github.com/DataDog/test-infra-definitions/scenarios/aws/fakeintake"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"

	"github.com/DataDog/opentelemetry-collector-contrib/e2e-tests/otel-collector/helm"
	"github.com/DataDog/opentelemetry-collector-contrib/e2e-tests/otel-collector/otelparams"
)

const (
	provisionerBaseID = "aws-otel-collector-kind-"
	defaultVMName     = "otel-collector-kind"
)

// ProvisionerOption is a function that modifies the ProvisionerParams
type ProvisionerOption func(*ProvisionerParams) error

type ProvisionerParams struct {
	name        string
	otelOptions []otelparams.Option
	extraParams runner.ConfigMap
}

func newProvisionerParams() *ProvisionerParams {
	return &ProvisionerParams{
		name:        defaultVMName,
		otelOptions: []otelparams.Option{},
	}
}

// WithOTelOptions sets the options for the OTel collector
func WithOTelOptions(opts ...otelparams.Option) ProvisionerOption {
	return func(params *ProvisionerParams) error {
		params.otelOptions = append(params.otelOptions, opts...)
		return nil
	}
}

// WithExtraParams sets the extra parameters for the provisioner
func WithExtraParams(params runner.ConfigMap) ProvisionerOption {
	return func(p *ProvisionerParams) error {
		p.extraParams = params
		return nil
	}
}

// Provisioner creates a new provisioner
func Provisioner(opts ...ProvisionerOption) e2e.TypedProvisioner[Kubernetes] {
	// We ALWAYS need to make a deep copy of `params`, as the provisioner can be called multiple times.
	// and it's easy to forget about it, leading to hard to debug issues.
	params := newProvisionerParams()
	_ = optional.ApplyOptions(params, opts)

	provisioner := e2e.NewTypedPulumiProvisioner(provisionerBaseID+params.name, func(ctx *pulumi.Context, env *Kubernetes) error {
		// We ALWAYS need to make a deep copy of `params`, as the provisioner can be called multiple times.
		// and it's easy to forget about it, leading to hard to debug issues.
		params := newProvisionerParams()
		_ = optional.ApplyOptions(params, opts)

		return RunFunc(ctx, env, params)
	}, params.extraParams)

	return provisioner
}

// RunFunc is the Pulumi run function that runs the provisioner
func RunFunc(ctx *pulumi.Context, env *Kubernetes, params *ProvisionerParams) error {
	awsEnv, err := aws.NewEnvironment(ctx)
	if err != nil {
		return err
	}

	host, err := ec2.NewVM(awsEnv, params.name)
	if err != nil {
		return err
	}

	installEcrCredsHelperCmd, err := ec2.InstallECRCredentialsHelper(awsEnv, host)
	if err != nil {
		return err
	}

	kindCluster, err := kubeComp.NewKindCluster(&awsEnv, host, awsEnv.CommonNamer().ResourceName("kind"), params.name, awsEnv.KubernetesVersion(), utils.PulumiDependsOn(installEcrCredsHelperCmd))
	if err != nil {
		return err
	}

	err = kindCluster.Export(ctx, &env.KubernetesCluster.ClusterOutput)
	if err != nil {
		return err
	}

	kubeProvider, err := kubernetes.NewProvider(ctx, awsEnv.Namer.ResourceName("k8s-provider"), &kubernetes.ProviderArgs{
		EnableServerSideApply: pulumi.Bool(true),
		Kubeconfig:            kindCluster.KubeConfig,
	})
	if err != nil {
		return err
	}

	fi, err := fakeintake.NewECSFargateInstance(awsEnv, params.name)
	if err != nil {
		return err
	}
	fi.Export(ctx, &env.FakeIntake.FakeintakeOutput)

	otelOptions := []otelparams.Option{otelparams.WithPulumiResourceOptions(pulumi.Provider(kubeProvider)), otelparams.WithFakeintake(fi)}
	otelOptions = append(otelOptions, params.otelOptions...)
	otelCollector, err := helm.NewOTelCollector(&awsEnv, "otel-collector", otelOptions...)
	if err != nil {
		return err
	}

	otelCollector.Export(ctx, &env.OTelCollector.OTelCollectorOutput)

	return nil
}

// LocalRunFunc is the Pulumi run function that runs the provisioner
func LocalRunFunc(ctx *pulumi.Context, env *Kubernetes, params *ProvisionerParams) error {
	localEnv, err := local.NewEnvironment(ctx)
	if err != nil {
		return err
	}

	kindCluster, err := kubeComp.NewLocalKindCluster(&localEnv, localEnv.CommonNamer().ResourceName("kind"), params.name, localEnv.KubernetesVersion())
	if err != nil {
		return err
	}

	err = kindCluster.Export(ctx, &env.KubernetesCluster.ClusterOutput)
	if err != nil {
		return err
	}

	kubeProvider, err := kubernetes.NewProvider(ctx, localEnv.CommonNamer().ResourceName("k8s-provider"), &kubernetes.ProviderArgs{
		EnableServerSideApply: pulumi.Bool(true),
		Kubeconfig:            kindCluster.KubeConfig,
	})
	if err != nil {
		return err
	}

	fakeintake, err := fakeintakeComp.NewLocalDockerFakeintake(&localEnv, "fakeintake")
	if err != nil {
		return err
	}
	fakeintake.Export(ctx, &env.FakeIntake.FakeintakeOutput)

	otelOptions := []otelparams.Option{otelparams.WithPulumiResourceOptions(pulumi.Provider(kubeProvider)), otelparams.WithFakeintake(fakeintake)}
	otelOptions = append(otelOptions, params.otelOptions...)

	otelCollector, err := helm.NewOTelCollector(&localEnv, "otel-collector", otelOptions...)
	if err != nil {
		return err
	}

	otelCollector.Export(ctx, &env.OTelCollector.OTelCollectorOutput)
	return nil
}
