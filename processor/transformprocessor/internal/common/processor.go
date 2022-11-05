// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor/internal/common"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlresource"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlscope"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const (
	Resource  string = "resource"
	Scope     string = "scope"
	Trace     string = "trace"
	DataPoint string = "datapoint"
	Log       string = "log"
)

type ContextStatements struct {
	Context    string   `mapstructure:"context"`
	Statements []string `mapstructure:"statements"`
}

var _ TracesContext = &ResourceStatements{}
var _ LogsContext = &ResourceStatements{}

type ResourceStatements struct {
	Statements []*ottl.Statement[ottlresource.TransformContext]
}

func (r *ResourceStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		ctx := ottlresource.NewTransformContext(rspans.Resource())
		for _, statement := range r.Statements {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *ResourceStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		ctx := ottlresource.NewTransformContext(rlogs.Resource())
		for _, statement := range r.Statements {
			_, _, err := statement.Execute(ctx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

var _ TracesContext = &ScopeStatements{}
var _ LogsContext = &ScopeStatements{}

type ScopeStatements struct {
	Statements []*ottl.Statement[ottlscope.TransformContext]
}

func (s *ScopeStatements) ProcessTraces(td ptrace.Traces) error {
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rspans := td.ResourceSpans().At(i)
		for j := 0; j < rspans.ScopeSpans().Len(); j++ {
			sspans := rspans.ScopeSpans().At(j)
			ctx := ottlscope.NewTransformContext(sspans.Scope(), rspans.Resource())
			for _, statement := range s.Statements {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *ScopeStatements) ProcessLogs(td plog.Logs) error {
	for i := 0; i < td.ResourceLogs().Len(); i++ {
		rlogs := td.ResourceLogs().At(i)
		for j := 0; j < rlogs.ScopeLogs().Len(); j++ {
			slogs := rlogs.ScopeLogs().At(j)
			ctx := ottlscope.NewTransformContext(slogs.Scope(), rlogs.Resource())
			for _, statement := range s.Statements {
				_, _, err := statement.Execute(ctx)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type parserCollection struct {
	settings       component.TelemetrySettings
	resourceParser ottl.Parser[ottlresource.TransformContext]
	scopeParser    ottl.Parser[ottlscope.TransformContext]
}

type baseContext interface {
	ProcessTraces(td ptrace.Traces) error
	ProcessLogs(td plog.Logs) error
}

func (pc parserCollection) parseCommonContextStatements(contextStatement ContextStatements) (baseContext, error) {
	switch contextStatement.Context {
	case Resource:
		statements, err := pc.resourceParser.ParseStatements(contextStatement.Statements)
		if err != nil {
			return nil, err
		}
		return &ResourceStatements{
			Statements: statements,
		}, nil
	case Scope:
		statements, err := pc.scopeParser.ParseStatements(contextStatement.Statements)
		if err != nil {
			return nil, err
		}
		return &ScopeStatements{
			Statements: statements,
		}, nil
	default:
		return nil, nil
	}
}
