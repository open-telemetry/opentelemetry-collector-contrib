// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package e2etests

import (
	"context"
	"flag"
	"fmt"
	"testing"

	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/runner"
	otelcollector "github.com/DataDog/opentelemetry-collector-contrib/e2e-tests/otel-collector"

	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/e2e"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO: Can be improved. On datadog-agent we use python code to run the e2e tests and pass the docker secret as an env variable
var dockerSecret *string = flag.String("docker_secret", "", "Docker secret to use for the tests")

type otelSuite struct {
	e2e.BaseSuite[otelcollector.Kubernetes]
}

// TestVMSuite runs tests for the VM interface to ensure its implementation is correct.
func TestVMSuite(t *testing.T) {
	extraParams := runner.ConfigMap{}
	extraParams.Set("ddagent:imagePullRegistry", "669783387624.dkr.ecr.us-east-1.amazonaws.com", false)
	extraParams.Set("ddagent:imagePullPassword", *dockerSecret, true)
	extraParams.Set("ddagent:imagePullUsername", "AWS", false)

	suiteParams := []e2e.SuiteOption{e2e.WithProvisioner(otelcollector.Provisioner(otelcollector.WithOTelOptions(), otelcollector.WithExtraParams(extraParams)))}

	e2e.Run(t, &otelSuite{}, suiteParams...)
}

func (v *otelSuite) TestExecute() {
	res, _ := v.Env().KubernetesCluster.Client().CoreV1().Pods("default").List(context.TODO(), v1.ListOptions{})
	for _, pod := range res.Items {
		v.T().Logf("Pod: %s", pod.Name)
	}
	va, err := v.Env().FakeIntake.Client().GetMetricNames()
	assert.NoError(v.T(), err)
	fmt.Printf("metriiiics: %v", va)
	lo, err := v.Env().FakeIntake.Client().FilterLogs("")
	for _, l := range lo {
		fmt.Printf("logs	: %v", l.Tags)
	}

	assert.NoError(v.T(), err)
	fmt.Printf("logs: %v", lo)
	// assert.Equal(v.T(), 1, len(res.Items))

	assert.Equal(v.T(), 1, len(res.Items))
}
