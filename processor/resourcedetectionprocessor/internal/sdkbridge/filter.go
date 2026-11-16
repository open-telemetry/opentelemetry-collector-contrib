// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sdkbridge // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"

import (
	"reflect"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
)

// RemoveDisabledAttributes drops from res every attribute not enabled in rac, an
// mdatagen-generated ResourceAttributesConfig. Keys come from the fields' mapstructure tags,
// so the mapping cannot drift from metadata.yaml.
//
// It occupies the same position as ResourceBuilder.Emit() and carries the same contract:
// only attributes declared in metadata.yaml survive. Dynamic attributes such as the ec2
// detector's "ec2.tag.<key>" must be added after this call, as they already are after Emit().
//
// Detectors that re-key an SDK attribute, or fan one out to several, must use their
// ResourceBuilder instead.
func RemoveDisabledAttributes(res pcommon.Resource, rac any) {
	enabled := enabledAttributes(rac)
	res.Attributes().RemoveIf(func(k string, _ pcommon.Value) bool {
		return !enabled[k]
	})
}

// SetEnabledAttributes sets the Enabled flag of every attribute in rac, a pointer to an
// mdatagen-generated ResourceAttributesConfig, to enabled(key). Keys come from the fields'
// mapstructure tags, as in RemoveDisabledAttributes. It does nothing if rac is not a pointer.
func SetEnabledAttributes(rac any, enabled func(key string) bool) {
	if reflect.ValueOf(rac).Kind() != reflect.Pointer {
		return
	}
	forEachAttribute(rac, func(key string, flag reflect.Value) {
		flag.SetBool(enabled(key))
	})
}

func enabledAttributes(rac any) map[string]bool {
	enabled := map[string]bool{}
	forEachAttribute(rac, func(key string, flag reflect.Value) {
		enabled[key] = flag.Bool()
	})
	return enabled
}

// forEachAttribute calls fn with the key and the Enabled flag of every attribute in rac, an
// mdatagen-generated ResourceAttributesConfig or a pointer to one. The flag is settable when
// rac is a pointer.
func forEachAttribute(rac any, fn func(key string, flag reflect.Value)) {
	v := reflect.ValueOf(rac)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		// The tag may carry options (e.g. `key,squash`); only the name is the attribute key.
		key, _, _ := strings.Cut(t.Field(i).Tag.Get("mapstructure"), ",")
		if key == "" || key == "-" {
			continue
		}
		field := v.Field(i)
		if field.Kind() != reflect.Struct {
			continue
		}
		flag := field.FieldByName("Enabled")
		if !flag.IsValid() || flag.Kind() != reflect.Bool {
			continue
		}
		fn(key, flag)
	}
}
