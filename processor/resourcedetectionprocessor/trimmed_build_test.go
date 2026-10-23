// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build remove_all_resourcedetection_detectors

package resourcedetectionprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestTrimmedBuildRegistryFactories(t *testing.T) {
	for detectorType, factory := range availableDetectorFactories() {
		require.NotNil(t, factory, "factory should not be nil for %q", detectorType)
	}
}

func TestTrimmedBuildCreatesProcessorWithoutDetectors(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Detectors = nil

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	assert.NotNil(t, tp)
	require.ErrorContains(t, tp.Start(t.Context(), componenttest.NewNopHost()), "no detectors succeeded")
}

func TestTrimmedBuildRejectsDetectorThatIsNotCompiled(t *testing.T) {
	allDetectorTypes := []internal.DetectorType{
		"akamai", "aks", "alibaba_ecs", "azure", "azureappservice", "azurecontainerapps", "azurefunctions", "consul", "digitalocean", "docker",
		"dynatrace", "ec2", "ecs", "eks", "elastic_beanstalk", "env", "gcp", "heroku", "hetzner",
		"ibmcloud_classic", "ibmcloud_vpc", "k8s_api", "kubeadm", "lambda", "nova", "openshift",
		"oraclecloud", "scaleway", "system", "tencent_cvm", "upcloud", "vultr",
	}

	var disabled internal.DetectorType
	for _, detectorType := range allDetectorTypes {
		if _, ok := availableDetectorFactories()[detectorType]; !ok {
			disabled = detectorType
			break
		}
	}
	if disabled == "" {
		t.Skip("all detectors are compiled")
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	cfg.(*Config).Detectors = []string{string(disabled)}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorContains(t, err, "is not compiled into this binary")
	assert.Nil(t, tp)
}

func TestTrimmedBuildDefaultRequiresEnvDetector(t *testing.T) {
	if _, ok := availableDetectorFactories()[internal.DetectorType("env")]; ok {
		t.Skip("env is compiled")
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.ErrorContains(t, err, `detector "env" is not compiled into this binary`)
	assert.Nil(t, tp)
}
