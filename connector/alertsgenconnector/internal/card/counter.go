// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package card

import "sync"

type Counter struct { limit int; mu sync.Mutex; set map[string]struct{} }

func New(limit int) *Counter { return &Counter{ limit:limit, set: map[string]struct{}{} } }

func (c *Counter) Add(key string) bool { c.mu.Lock(); defer c.mu.Unlock(); if _, ok := c.set[key]; ok { return true } ; if len(c.set) >= c.limit { return false } ; c.set[key]=struct{}{}; return true }
