// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package admission2 // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow/admission2"

// notification.N is a minimal Go version of absl::Notification:
//
//	https://github.com/abseil/abseil-cpp/blob/master/absl/synchronization/notification.h
//
// Use New() to construct a notification object (the zero value is not
// usable).
type N struct {
	c chan struct{}
}

func newNotification() N {
	return N{c: make(chan struct{})}
}

func (n *N) Notify() {
	close(n.c)
}

func (n *N) HasBeenNotified() bool {
	select {
	case <-n.c:
		return true
	default:
		return false
	}
}

func (n *N) WaitForNotification() {
	<-n.c
}

// Chan allows a caller to wait for the notification as part of a
// select statement. Outside of a select statement, prefer writing
// WaitForNotification().
func (n *N) Chan() <-chan struct{} {
	return n.c
}
