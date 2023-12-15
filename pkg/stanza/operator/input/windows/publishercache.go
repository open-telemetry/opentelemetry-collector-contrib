// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows
// +build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"errors"
)

type publisherCache struct {
	cache map[string]Publisher
}

func newPublisherCache() publisherCache {
	return publisherCache{
		cache: make(map[string]Publisher),
	}
}

func (c *publisherCache) get(provider string) (publisher Publisher, openPublisherErr error) {
	publisher, ok := c.cache[provider]
	if ok {
		return publisher, nil
	}

	publisher = NewPublisher()
	err := publisher.Open(provider)

	// Always store the publisher even if there was an error opening it.
	c.cache[provider] = publisher

	return publisher, err
}

func (c *publisherCache) evictAll() error {
	var errs error
	for _, publisher := range c.cache {
		if publisher.Valid() {
			if err := publisher.Close(); err != nil {
				errs = errors.Join(errs, err)
			}
		}
	}

	c.cache = make(map[string]Publisher)
	return errs
}
