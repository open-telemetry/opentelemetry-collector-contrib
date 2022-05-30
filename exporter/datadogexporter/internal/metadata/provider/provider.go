package provider

import (
	"context"
)

// HostnameProvider of a hostname from a given place.
type HostnameProvider interface {
	// Metadata gets host metadata from provider.
	Hostname(ctx context.Context) (string, error)
}
