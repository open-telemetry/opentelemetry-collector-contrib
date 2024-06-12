package supervisor

import "os"

// pidProvider provides the PID of the current process
type pidProvider interface {
	PID() int
}

type defaultPIDProvider struct{}

func (defaultPIDProvider) PID() int {
	return os.Getpid()
}
