// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ctxlog

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type compileTimeTestContext struct {
	log   plog.LogRecord
	cache pcommon.Map
}

func (c *compileTimeTestContext) GetLogRecord() plog.LogRecord {
	return c.log
}

func (c *compileTimeTestContext) GetCache() pcommon.Map {
	return c.cache
}

var _ interface {
	Get(context.Context, *compileTimeTestContext) (any, error)
	Set(context.Context, *compileTimeTestContext, any) error
	GetStringLike(context.Context, *compileTimeTestContext) (*string, bool, error)
} = bodyGetSetter[*compileTimeTestContext]{}
