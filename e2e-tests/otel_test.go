// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package e2etests

import (
	"context"
	otelcollector "e2e-tests/otel-collector"
	"testing"

	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/e2e"
	"github.com/DataDog/datadog-agent/test/new-e2e/pkg/environments"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type vmSuite struct {
	e2e.BaseSuite[environments.Kubernetes]
}

// TestVMSuite runs tests for the VM interface to ensure its implementation is correct.
func TestVMSuite(t *testing.T) {
	suiteParams := []e2e.SuiteOption{e2e.WithProvisioner(otelcollector.Provisioner())}

	e2e.Run(t, &vmSuite{}, suiteParams...)
}

func (v *vmSuite) TestExecute() {
	res, _ := v.Env().KubernetesCluster.Client().CoreV1().Pods("default").List(context.TODO(), v1.ListOptions{})
	for _, pod := range res.Items {
		v.T().Logf("Pod: %s", pod.Name)
	}
	assert.Equal(v.T(), 1, len(res.Items))
}
