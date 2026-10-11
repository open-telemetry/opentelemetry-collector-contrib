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

func enabledAttributes(rac any) map[string]bool {
	v := reflect.ValueOf(rac)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	enabled := make(map[string]bool, t.NumField())
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
		enabled[key] = flag.Bool()
	}
	return enabled
}
