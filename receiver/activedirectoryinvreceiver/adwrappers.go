// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

// Client is an interface for an Active Directory client
type Client interface {
	Open(path string) (Container, error)
}

// Container is an interface for an Active Directory container
type Container interface {
	ToObject() (Object, error)
	Close()
	Children() (ObjectIter, error)
}

// Object is an interface for an Active Directory object
type Object interface {
	Attrs(key string) ([]any, error)
	ToContainer() (Container, error)
}

// ObjectIter is an interface for an Active Directory object iterator
type ObjectIter interface {
	Next() (Object, error)
	Close()
}

// RuntimeInfo is an interface for runtime information
type RuntimeInfo interface {
	SupportedOS() bool
}
