package logs

import (
	"context"
	"github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface"
)

type service struct {
	name string
}

var _ hostnameinterface.Component = (*service)(nil)

// Get returns the hostname.
func (hs *service) Get(context.Context) (string, error) {
	return hs.name, nil
}

// GetSafe returns the hostname, or 'unknown host' if anything goes wrong.
func (hs *service) GetSafe(ctx context.Context) string {
	name, err := hs.Get(ctx)
	if err != nil {
		return "unknown host"
	}
	return name
}

// GetWithProvider returns the hostname for the Agent and the provider that was use to retrieve it.
func (hs *service) GetWithProvider(context.Context) (hostnameinterface.Data, error) {
	return hostnameinterface.Data{
		Hostname: hs.name,
		Provider: "",
	}, nil
}

// NewHostnameService creates a new instance of the component hostname
func NewHostnameService(name string) hostnameinterface.Component {
	return &service{
		name: name,
	}
}
