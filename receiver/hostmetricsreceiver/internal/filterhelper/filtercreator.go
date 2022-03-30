package filterhelper

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

const (
	excludeKey = "exclude"
	includeKey = "include"
)

// NewIncludeFilterHelper creates a FilterSet based on a config
func NewIncludeFilterHelper(items []string, filterSet *filterset.Config, typ string) (filterset.FilterSet, error) {
	return newFilterHelper(items, filterSet, includeKey, typ)
}

// NewExcludeFilterHelper creates a FilterSet based on a config
func NewExcludeFilterHelper(items []string, filterSet *filterset.Config, typ string) (filterset.FilterSet, error) {
	return newFilterHelper(items, filterSet, excludeKey, typ)
}

func newFilterHelper(items []string, filterSet *filterset.Config, typ string, filterType string) (filterset.FilterSet, error) {
	var err error
	var filter filterset.FilterSet

	if len(items) > 0 {
		filter, err = filterset.CreateFilterSet(items, filterSet)
		if err != nil {
			return nil, fmt.Errorf("error creating %s %s filters: %w", filterType, typ, err)
		}
	}
	return filter, nil
}
