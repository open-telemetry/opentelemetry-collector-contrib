// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package geoipprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor"

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/multierr"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor/internal/provider"
)

var (
	errIPNotFound        = errors.New("no IP address found in the resource attributes")
	errUnspecifiedIP     = errors.New("unspecified address")
	errUnspecifiedSource = errors.New("no source attributes defined")
)

// newGeoIPProcessor creates a new instance of geoIPProcessor with the specified fields.
type geoIPProcessor struct {
	providers []provider.GeoIPProvider
	logger    *zap.Logger

	cfg *Config
}

func newGeoIPProcessor(processorConfig *Config, providers []provider.GeoIPProvider, params processor.Settings) *geoIPProcessor {
	return &geoIPProcessor{
		providers: providers,
		cfg:       processorConfig,
		logger:    params.Logger,
	}
}

// parseIP parses a string to a net.IP type and returns an error if the IP is invalid or unspecified.
func parseIP(strIP string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(strIP)
	if err != nil {
		return netip.Addr{}, err
	} else if ip.IsUnspecified() {
		return netip.Addr{}, fmt.Errorf("%w address: %s", errUnspecifiedIP, strIP)
	}
	return ip, nil
}

// ipFromAttributes extracts an IP address from the given attributes based on the specified fields.
// It returns the first IP address if found, or an error if no valid IP address is found.
func ipFromAttributes(attributes []attribute.Key, resource pcommon.Map) (netip.Addr, error) {
	for _, attr := range attributes {
		if ipField, found := resource.Get(string(attr)); found {
			// The attribute might contain a domain name. Skip any net.ParseIP error until we have a fine-grained error propagation strategy.
			// TODO: propagate an error once error_mode configuration option is available (e.g. transformprocessor)
			ipAttribute, err := parseIP(ipField.AsString())
			if err == nil {
				return ipAttribute, nil
			}
		}
	}

	return netip.Addr{}, errIPNotFound
}

// geoLocation fetches geolocation information for the given IP address using the configured providers.
// It returns a set of attributes containing the geolocation data, or an error if the location could not be determined.
func (g *geoIPProcessor) geoLocation(ctx context.Context, ip netip.Addr) (attribute.Set, error) {
	allAttributes := &attribute.Set{}
	for _, geoProvider := range g.providers {
		geoAttributes, err := geoProvider.Location(ctx, ip)
		if err != nil {
			// continue if no metadata is found
			if errors.Is(err, provider.ErrNoMetadataFound) {
				g.logger.Debug(err.Error(), zap.String("IP", ip.String()))
				continue
			}
			return attribute.Set{}, err
		}
		*allAttributes = attribute.NewSet(append(allAttributes.ToSlice(), geoAttributes.ToSlice()...)...)
	}

	return *allAttributes, nil
}

// processAttributes processes a pcommon.Map by adding geolocation attributes based on the found IP address.
func (g *geoIPProcessor) processAttributes(ctx context.Context, metadata pcommon.Map) error {
	ipAddr, err := ipFromAttributes(g.cfg.Attributes, metadata)
	if err != nil {
		// TODO: log IP error not found
		if errors.Is(err, errIPNotFound) {
			return nil
		}
		return err
	}

	attributes, err := g.geoLocation(ctx, ipAddr)
	if err != nil {
		return err
	}

	for _, geoAttr := range attributes.ToSlice() {
		switch geoAttr.Value.Type() {
		case attribute.FLOAT64:
			metadata.PutDouble(string(geoAttr.Key), geoAttr.Value.AsFloat64())
		case attribute.STRING:
			metadata.PutStr(string(geoAttr.Key), geoAttr.Value.AsString())
		}
	}

	return nil
}

func (g *geoIPProcessor) shutdown(ctx context.Context) error {
	var errs error
	for _, geoProvider := range g.providers {
		err := geoProvider.Close(ctx)
		if err != nil {
			errs = multierr.Append(errs, err)
		}
	}

	return errs
}
