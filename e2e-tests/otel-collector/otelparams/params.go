package otelparams

import (
	"github.com/DataDog/test-infra-definitions/components/datadog/fakeintake"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Params struct {
	Fakeintake            *fakeintake.Fakeintake
	HelmValues            pulumi.AssetOrArchiveArray
	PulumiResourceOptions []pulumi.ResourceOption
}

type Option = func(*Params) error

func NewParams(opts ...Option) (*Params, error) {
	p := &Params{}
	for _, o := range opts {
		if err := o(p); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func WithHelmValues(values string) Option {
	return func(p *Params) error {
		p.HelmValues = append(p.HelmValues, pulumi.NewStringAsset(values))
		return nil
	}
}

func WithPulumiResourceOptions(opts ...pulumi.ResourceOption) Option {
	return func(p *Params) error {
		p.PulumiResourceOptions = append(p.PulumiResourceOptions, opts...)
		return nil
	}
}

func WithFakeintake(fakeintake *fakeintake.Fakeintake) Option {
	return func(p *Params) error {
		p.Fakeintake = fakeintake
		return nil
	}
}
