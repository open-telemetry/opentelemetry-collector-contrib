// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package activedirectoryinvreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectoryinvreceiver"

import (
	"fmt"
	"runtime"

	adsi "github.com/go-adsi/adsi"
)

var _ Client = (*adsiClient)(nil)
var _ Container = (*adsiContainer)(nil)
var _ Object = (*adObject)(nil)
var _ ObjectIter = (*adObjectIter)(nil)
var _ RuntimeInfo = (*adRuntimeInfo)(nil)

type adsiClient struct{}

func (c *adsiClient) Open(path string) (Container, error) {
	client, err := adsi.NewClient()
	if err != nil {
		return nil, err
	}
	ldapPath := fmt.Sprintf("LDAP://%s", path)
	root, err := client.Open(ldapPath)
	if err != nil {
		return nil, err
	}
	rootContainer, err := root.ToContainer()
	if err != nil {
		return nil, err
	}
	return &adsiContainer{rootContainer}, nil
}

// adsiContainer is a wrapper for an Active Directory container
type adsiContainer struct {
	windowsADContainer *adsi.Container
}

// ToObject converts an Active Directory container to an Active Directory object
func (c *adsiContainer) ToObject() (Object, error) {
	object, err := c.windowsADContainer.ToObject()
	if err != nil {
		return nil, err
	}
	return &adObject{object}, nil
}

// Close closes an Active Directory container
func (c *adsiContainer) Close() {
	c.windowsADContainer.Close()
}

// Children returns the children of an Active Directory container
func (c *adsiContainer) Children() (ObjectIter, error) {
	objectIter, err := c.windowsADContainer.Children()
	if err != nil {
		return nil, err
	}
	return &adObjectIter{objectIter}, nil
}

// adObject is a wrapper for an Active Directory object
type adObject struct {
	windowsADObject *adsi.Object
}

// Attrs returns the attributes of an Active Directory object
func (o *adObject) Attrs(key string) ([]interface{}, error) {
	return o.windowsADObject.Attr(key)
}

// ToContainer converts an Active Directory object to an Active Directory container
func (o *adObject) ToContainer() (Container, error) {
	container, err := o.windowsADObject.ToContainer()
	if err != nil {
		return nil, err
	}
	return &adsiContainer{container}, nil
}

// adObjectIter is a wrapper for an Active Directory object iterator
type adObjectIter struct {
	windowsADObjectIter *adsi.ObjectIter
}

// Next returns the next Active Directory object in the iterator
func (o *adObjectIter) Next() (Object, error) {
	obj, err := o.windowsADObjectIter.Next()
	if err != nil {
		return nil, err
	}
	return &adObject{obj}, nil
}

// Close closes an Active Directory object iterator
func (o *adObjectIter) Close() {
	o.windowsADObjectIter.Close()
}

// adRuntimeInfo is a wrapper for runtime information
type adRuntimeInfo struct{}

// SupportedOS returns whether the runtime is supported
func (r *adRuntimeInfo) SupportedOS() bool {
	return runtime.GOOS == "windows"
}
