// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dorisexporter // import "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter"

import (
	"fmt"
	"testing"
	"time"
)

func TestLocalTimeZone(t *testing.T) {
	fmt.Println(time.Now().In(time.Local).Format("Z07:00"))
}

func TestFormatUint(t *testing.T) {
	a := "aaa"
	var b uint64 = 1000001
	s := fmt.Sprintf("%s_%d", a, b)
	fmt.Println(s)
}

func TestFormatTime(t *testing.T) {
	tm := time.Now()
	fmt.Println(tm.Format("2006-01-02 15:04:05.999999999"))
}
