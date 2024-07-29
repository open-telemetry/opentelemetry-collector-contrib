package otelcollector

import (
	"e2e-tests/otel-collector/helm"

	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/e2e"
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/environments"
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/runner"
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/utils/optional"
	"github.com/DataDog/test-infra-definitions/common/utils"
	"github.com/DataDog/test-infra-definitions/components/datadog/kubernetesagentparams"
	kubeComp "github.com/DataDog/test-infra-definitions/components/kubernetes"
	"github.com/DataDog/test-infra-definitions/resources/aws"
	"github.com/DataDog/test-infra-definitions/resources/local"
	"github.com/DataDog/test-infra-definitions/scenarios/aws/ec2"
	"github.com/DataDog/test-infra-definitions/scenarios/aws/fakeintake"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	provisionerBaseID = "aws-otel-collector-kind-"
	defaultVMName     = "otel-collector-kind"
)

// ProvisionerOption is a function that modifies the ProvisionerParams
type ProvisionerOption func(*ProvisionerParams) error

type ProvisionerParams struct {
	name              string
	vmOptions         []ec2.VMOption
	agentOptions      []kubernetesagentparams.Option
	fakeintakeOptions []fakeintake.Option
	extraConfigParams runner.ConfigMap

	eksLinuxNodeGroup        bool
	eksLinuxARMNodeGroup     bool
	eksBottlerocketNodeGroup bool
	eksWindowsNodeGroup      bool
	awsEnv                   *aws.Environment
	deployDogstatsd          bool
}

func newProvisionerParams() *ProvisionerParams {
	return &ProvisionerParams{
		name:              defaultVMName,
		vmOptions:         []ec2.VMOption{},
		agentOptions:      []kubernetesagentparams.Option{},
		fakeintakeOptions: []fakeintake.Option{},
		extraConfigParams: runner.ConfigMap{},

		eksLinuxNodeGroup:        false,
		eksLinuxARMNodeGroup:     false,
		eksBottlerocketNodeGroup: false,
		eksWindowsNodeGroup:      false,
		deployDogstatsd:          false,
	}
}

// Provisioner creates a new provisioner
func Provisioner(opts ...ProvisionerOption) e2e.TypedProvisioner[environments.Kubernetes] {
	// We ALWAYS need to make a deep copy of `params`, as the provisioner can be called multiple times.
	// and it's easy to forget about it, leading to hard to debug issues.
	params := newProvisionerParams()
	_ = optional.ApplyOptions(params, opts)

	provisioner := e2e.NewTypedPulumiProvisioner(provisionerBaseID+params.name, func(ctx *pulumi.Context, env *environments.Kubernetes) error {
		// We ALWAYS need to make a deep copy of `params`, as the provisioner can be called multiple times.
		// and it's easy to forget about it, leading to hard to debug issues.
		params := newProvisionerParams()
		_ = optional.ApplyOptions(params, opts)

		return LocalRunFunc(ctx, env, params)
	}, params.extraConfigParams)

	return provisioner
}

// RunFunc is the Pulumi run function that runs the provisioner
func RunFunc(ctx *pulumi.Context, env *environments.Kubernetes, params *ProvisionerParams) error {
	awsEnv, err := aws.NewEnvironment(ctx)
	if err != nil {
		return err
	}

	host, err := ec2.NewVM(awsEnv, params.name, params.vmOptions...)
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

	_, err = kubernetes.NewProvider(ctx, awsEnv.Namer.ResourceName("k8s-provider"), &kubernetes.ProviderArgs{
		EnableServerSideApply: pulumi.Bool(true),
		Kubeconfig:            kindCluster.KubeConfig,
	})
	if err != nil {
		return err
	}

	env.Agent = nil
	return nil
}

// LocalRunFunc is the Pulumi run function that runs the provisioner
func LocalRunFunc(ctx *pulumi.Context, env *environments.Kubernetes, params *ProvisionerParams) error {
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

	helm.NewOtelCollector(&localEnv, "otel-collector", kubeProvider)

	env.Agent = nil
	env.FakeIntake = nil
	return nil
}
