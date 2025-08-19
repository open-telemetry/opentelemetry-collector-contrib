// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"sync"
	"time"
	"go.uber.org/zap"
)

type Store struct { cfg interface{}; log *zap.Logger; mu sync.Mutex; activeSince map[string]time.Time; active map[string]int64 }

func New(cfg interface{}, log *zap.Logger) *Store { return &Store{ cfg:cfg, log:log, activeSince: map[string]time.Time{}, active: map[string]int64{} } }
