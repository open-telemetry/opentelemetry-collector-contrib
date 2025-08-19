// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package alertsgenconnector

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func mapSeverity(m SeverityMap, reasonOrLabels ...string) string { text := strings.ToLower(strings.Join(reasonOrLabels, " ")) ; in := func(list []string) bool { for _, k := range list { if k!="" && strings.Contains(text, strings.ToLower(k)) { return true } } ; return false } ; switch { case in(m.Fatal): return "fatal"; case in(m.Error): return "error"; case in(m.Warning): return "warning"; case in(m.Info): return "info"; default: return "unknown" } }

func putLabels(attrs pcommon.Map, labels map[string]string, max int) { if len(labels)==0 { return } ; keys := make([]string,0,len(labels)); for k := range labels { keys = append(keys,k) } ; sort.Strings(keys) ; written := 0 ; for _, k := range keys { if written>=max { break } ; attrs.PutStr("labels."+k, labels[k]); written++ } ; if written < len(labels) { h:=sha256.New(); for _, k := range keys[written:] { h.Write([]byte(k)); h.Write([]byte{0}); h.Write([]byte(labels[k])); h.Write([]byte{0}) } ; attrs.PutStr("labels._spill_sha256", fmt.Sprintf("%x", h.Sum(nil))) ; attrs.PutInt("labels._spill_count", int64(len(labels)-written)) } }
