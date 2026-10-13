// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/partitioningprocessor"

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/pprofile"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlotelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

type profilesPartitioner interface {
	partitionProfiles(ctx context.Context, pd pprofile.Profiles) ([]partitionedProfiles, error)
}

type partitionedProfiles = partitioned[pprofile.Profiles]

// otelcolProfilesPartitioner evaluates expressions against the request-scoped otelcol context.
type otelcolProfilesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlotelcol.TransformContext]
}

func (p *otelcolProfilesPartitioner) partitionProfiles(ctx context.Context, pd pprofile.Profiles) ([]partitionedProfiles, error) {
	if pd.ResourceProfiles().Len() == 0 {
		return nil, nil
	}
	values := make([]keyValue, len(p.expressions))
	if err := evaluateStringExpressions(ctx, p.expressions, ottlotelcol.NewTransformContext(), values); err != nil {
		return nil, err
	}
	return []partitionedProfiles{{values: values, data: pd}}, nil
}

// resourceProfilesPartitioner evaluates expressions at the ResourceProfiles level.
type resourceProfilesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlresource.TransformContext]
}

func (p *resourceProfilesPartitioner) partitionProfiles(ctx context.Context, pd pprofile.Profiles) ([]partitionedProfiles, error) {
	g := newGrouper(len(p.expressions))
	ids := make([]int32, 0, pd.ResourceProfiles().Len())
	for _, rp := range pd.ResourceProfiles().All() {
		id, err := assign(ctx, g, p.expressions, ottlresource.NewTransformContext(rp.Resource(), rp))
		if err != nil {
			return nil, err
		}
		ids = append(ids, int32(id))
	}
	if parts, ok := single(g, pd); ok {
		return parts, nil
	}

	parts := newPartitions(g, func() pprofile.Profiles { return newPartitionProfiles(pd.Dictionary()) })
	for i, rp := range pd.ResourceProfiles().All() {
		rp.MoveTo(parts[ids[i]].data.ResourceProfiles().AppendEmpty())
	}
	return parts, nil
}

// scopeProfilesPartitioner evaluates expressions at the ScopeProfiles level.
type scopeProfilesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlscope.TransformContext]
}

func (p *scopeProfilesPartitioner) partitionProfiles(ctx context.Context, pd pprofile.Profiles) ([]partitionedProfiles, error) {
	g := newGrouper(len(p.expressions))
	var ids []int32
	for _, rp := range pd.ResourceProfiles().All() {
		for _, sp := range rp.ScopeProfiles().All() {
			id, err := assign(ctx, g, p.expressions, ottlscope.NewTransformContext(sp.Scope(), rp.Resource(), sp, rp))
			if err != nil {
				return nil, err
			}
			ids = append(ids, int32(id))
		}
	}
	if parts, ok := single(g, pd); ok {
		return parts, nil
	}

	parts := newPartitions(g, func() pprofile.Profiles { return newPartitionProfiles(pd.Dictionary()) })
	dests := make([]profilesDest, len(parts))
	for _, rp := range pd.ResourceProfiles().All() {
		for _, sp := range rp.ScopeProfiles().All() {
			id := ids[0]
			ids = ids[1:]
			sp.MoveTo(dests[id].resource(parts[id].data, rp).ScopeProfiles().AppendEmpty())
		}
	}
	return parts, nil
}

// profileProfilesPartitioner evaluates expressions at the Profile level.
type profileProfilesPartitioner struct {
	expressions []*ottl.ValueExpression[*ottlprofile.TransformContext]
}

func (p *profileProfilesPartitioner) partitionProfiles(ctx context.Context, pd pprofile.Profiles) ([]partitionedProfiles, error) {
	dict := pd.Dictionary()
	g := newGrouper(len(p.expressions))
	var ids []int32
	for _, rp := range pd.ResourceProfiles().All() {
		for _, sp := range rp.ScopeProfiles().All() {
			for _, profile := range sp.Profiles().All() {
				id, err := assign(ctx, g, p.expressions, ottlprofile.NewTransformContext(rp, sp, profile, dict))
				if err != nil {
					return nil, err
				}
				ids = append(ids, int32(id))
			}
		}
	}
	if parts, ok := single(g, pd); ok {
		return parts, nil
	}

	parts := newPartitions(g, func() pprofile.Profiles { return newPartitionProfiles(pd.Dictionary()) })
	dests := make([]profilesDest, len(parts))
	for _, rp := range pd.ResourceProfiles().All() {
		for _, sp := range rp.ScopeProfiles().All() {
			for _, profile := range sp.Profiles().All() {
				id := ids[0]
				ids = ids[1:]
				profile.MoveTo(dests[id].scope(parts[id].data, rp, sp).Profiles().AppendEmpty())
			}
		}
	}
	return parts, nil
}

// profilesDest tracks a partition's destination containers for the source
// resource and scope currently being visited. Sources are visited in order
// and never revisited, so only the most recent ones can match.
type profilesDest struct {
	srcRP, rp pprofile.ResourceProfiles
	srcSP, sp pprofile.ScopeProfiles
}

func (d *profilesDest) resource(data pprofile.Profiles, src pprofile.ResourceProfiles) pprofile.ResourceProfiles {
	if d.srcRP != src {
		d.srcRP = src
		d.rp = data.ResourceProfiles().AppendEmpty()
		src.Resource().CopyTo(d.rp.Resource())
		d.rp.SetSchemaUrl(src.SchemaUrl())
	}
	return d.rp
}

func (d *profilesDest) scope(data pprofile.Profiles, srcRP pprofile.ResourceProfiles, src pprofile.ScopeProfiles) pprofile.ScopeProfiles {
	if d.srcSP != src {
		d.srcSP = src
		d.sp = d.resource(data, srcRP).ScopeProfiles().AppendEmpty()
		src.Scope().CopyTo(d.sp.Scope())
		d.sp.SetSchemaUrl(src.SchemaUrl())
	}
	return d.sp
}

func newPartitionProfiles(dict pprofile.ProfilesDictionary) pprofile.Profiles {
	profiles := pprofile.NewProfiles()
	dict.CopyTo(profiles.Dictionary())
	return profiles
}

func newProfilesPartitioner(expressions []string, settings component.TelemetrySettings) (profilesPartitioner, error) {
	otelcolParser, err := ottlotelcol.NewParser(
		ottlfuncs.StandardConverters[*ottlotelcol.TransformContext](),
		settings,
		ottlotelcol.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	resourceParser, err := ottlresource.NewParser(
		ottlfuncs.StandardConverters[*ottlresource.TransformContext](),
		settings,
		ottlresource.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	scopeParser, err := ottlscope.NewParser(
		ottlfuncs.StandardConverters[*ottlscope.TransformContext](),
		settings,
		ottlscope.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	profileParser, err := ottlprofile.NewParser(
		ottlfuncs.StandardConverters[*ottlprofile.TransformContext](),
		settings,
		ottlprofile.EnablePathContextNames(),
	)
	if err != nil {
		return nil, err
	}

	pc, err := ottl.NewParserCollection[profilesPartitioner](
		settings,
		ottl.WithParserCollectionContext[*ottlotelcol.TransformContext, profilesPartitioner](
			ottlotelcol.ContextName,
			&otelcolParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[profilesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlotelcol.TransformContext],
			) (profilesPartitioner, error) {
				return &otelcolProfilesPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlresource.TransformContext, profilesPartitioner](
			ottlresource.ContextName,
			&resourceParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[profilesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlresource.TransformContext],
			) (profilesPartitioner, error) {
				return &resourceProfilesPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlscope.TransformContext, profilesPartitioner](
			ottlscope.ContextName,
			&scopeParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[profilesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlscope.TransformContext],
			) (profilesPartitioner, error) {
				return &scopeProfilesPartitioner{expressions: parsed}, nil
			}),
		),
		ottl.WithParserCollectionContext[*ottlprofile.TransformContext, profilesPartitioner](
			ottlprofile.ContextName,
			&profileParser,
			ottl.WithValueExpressionConverter(func(
				_ *ottl.ParserCollection[profilesPartitioner],
				_ ottl.ValueExpressionsGetter,
				parsed []*ottl.ValueExpression[*ottlprofile.TransformContext],
			) (profilesPartitioner, error) {
				return &profileProfilesPartitioner{expressions: parsed}, nil
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	return pc.ParseValueExpressions(ottl.NewValueExpressionsGetter(expressions))
}
