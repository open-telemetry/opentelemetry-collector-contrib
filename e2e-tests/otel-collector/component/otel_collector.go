// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"github.com/DataDog/test-infra-definitions/components"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type OTelCollector struct {
	pulumi.ResourceState
	components.Component
	LabelSelectors pulumi.Map `pulumi:"labelSelectors"`
}

func (c *OTelCollector) Export(ctx *pulumi.Context, out *OTelCollectorOutput) error {
	return components.Export(ctx, c, out)
}

type OTelCollectorOutput struct {
	components.JSONImporter
	LabelSelectors map[string]string `json:"labelSelectors"`
}
