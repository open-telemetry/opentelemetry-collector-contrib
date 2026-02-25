// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package windows // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"

import (
	"go.uber.org/multierr"
)

type publisherCache struct {
	cache map[string]Publisher
}

func newPublisherCache() publisherCache {
	return publisherCache{
		cache: make(map[string]Publisher),
	}
}

func (c *publisherCache) get(provider string) (Publisher, error) {
	publisher, ok := c.cache[provider]
	if ok {
		return publisher, nil
	}

	var err error
	publisher = NewPublisher()
	if provider != "" {
		// If the provider is empty, there is nothing to be formatted on the event
		// keep the invalid publisher in the cache. See issue #35135
		err = publisher.Open(provider)
	}

	// Always store the publisher even if there was an error opening it.
	c.cache[provider] = publisher

	return publisher, err
}

func (c *publisherCache) evictAll() error {
	var errs error
	for _, publisher := range c.cache {
		if publisher.Valid() {
			errs = multierr.Append(errs, publisher.Close())
		}
	}

	c.cache = make(map[string]Publisher)
	return errs
}
