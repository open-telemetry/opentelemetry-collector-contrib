package logs

import (
	"github.com/DataDog/datadog-agent/pkg/logs/status/statusinterface"
)

// StatusProvider is the type for logs agent status methods
type StatusProvider struct{}

var _ statusinterface.Status = (*StatusProvider)(nil)

// AddGlobalWarning keeps track of a warning message to display on the status.
func (p StatusProvider) AddGlobalWarning(string, string) {
}

// RemoveGlobalWarning loses track of a warning message
// that does not need to be displayed on the status anymore.
func (p StatusProvider) RemoveGlobalWarning(string) {
}

// NewStatusProvider fetches the status and returns a service wrapping it
func NewStatusProvider() statusinterface.Status {
	return &StatusProvider{}
}
