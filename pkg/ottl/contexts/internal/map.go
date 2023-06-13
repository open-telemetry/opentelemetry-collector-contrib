// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"fmt"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func GetMapValue(m pcommon.Map, keys ottl.Key) (interface{}, error) {
	if keys.String() == nil {
		return nil, fmt.Errorf("non-string indexing is not supported")
	}

	val, ok := m.Get(*keys.String())
	if !ok {
		return nil, nil
	}

	return getIndexableValue(val, keys.Next())
}

func SetMapValue(m pcommon.Map, keys ottl.Key, val interface{}) error {
	s := keys.String()
	if s == nil {
		return fmt.Errorf("non-string indexing is not supported")
	}
	currentValue, ok := m.Get(*s)
	if !ok {
		currentValue = m.PutEmpty(*s)
	}
	return setIndexableValue(currentValue, val, keys.Next())
}
