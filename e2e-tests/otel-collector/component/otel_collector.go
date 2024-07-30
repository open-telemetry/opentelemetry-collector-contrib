package component

import (
	"github.com/DataDog/test-infra-definitions/components"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type OtelCollector struct {
	pulumi.ResourceState
	components.Component
	LabelSelectors pulumi.Map `pulumi:"labelSelectors"`
}

func (c *OtelCollector) Export(ctx *pulumi.Context, out *OtelCollectorOutput) error {
	return components.Export(ctx, c, out)
}

type OtelCollectorOutput struct {
	components.JSONImporter
	LabelSelectors map[string]string `json:"labelSelectors"`
}
