// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// source(Apache 2.0): https://github.com/DataDog/datadog-agent/blob/main/pkg/collector/python/datadog_agent.go

// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package postgresqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver"

import (
	"sync"

	"github.com/DataDog/datadog-agent/pkg/obfuscate"
)

var (
	obfuscator       *obfuscate.Obfuscator
	obfuscatorLoader sync.Once
)

// lazyInitObfuscator initializes the obfuscator the first time it is used.
func lazyInitObfuscator() *obfuscate.Obfuscator {
	obfuscatorLoader.Do(func() { obfuscator = obfuscate.NewObfuscator(obfuscate.Config{}) })
	return obfuscator
}

// ObfuscateSQL obfuscates & normalizes the provided SQL query, writing the error into errResult if the operation fails.
func obfuscateSQL(rawQuery string) (string, error) {
	obfuscatedQuery, err := lazyInitObfuscator().ObfuscateSQLStringWithOptions(rawQuery, &obfuscate.SQLConfig{})
	if err != nil {
		return "", err
	}

	return obfuscatedQuery.Query, nil
}

// Ending source(Apache 2.0): https://github.com/DataDog/datadog-agent/blob/main/pkg/collector/python/datadog_agent.go
