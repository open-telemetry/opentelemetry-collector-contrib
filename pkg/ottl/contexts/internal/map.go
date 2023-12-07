// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/internal"

import (
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func GetMapValue(m pcommon.Map, key ottl.Key) (interface{}, error) {
	if key == nil || key.String() == nil {
		return nil, fmt.Errorf("non-string indexing is not supported")
	}

	val, ok := m.Get(*key.String())
	if !ok {
		return nil, nil
	}

	return getIndexableValue(val, key.Next())
}

func SetMapValue(m pcommon.Map, key ottl.Key, val interface{}) error {
	if key == nil || key.String() == nil {
		return fmt.Errorf("non-string indexing is not supported")
	}
	s := key.String()
	currentValue, ok := m.Get(*s)
	if !ok {
		currentValue = m.PutEmpty(*s)
	}
	return setIndexableValue(currentValue, val, key.Next())
}
