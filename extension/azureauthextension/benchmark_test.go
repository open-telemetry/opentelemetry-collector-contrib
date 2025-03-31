// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func BenchmarkGetCurrentToken(b *testing.B) {
	auth := &authenticator{}
	auth.token.Store(azcore.AccessToken{Token: "test", ExpiresOn: time.Now().Add(time.Hour)})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = auth.getCurrentToken()
	}
}

func BenchmarkGetCurrentTokenParallel(b *testing.B) {
	auth := &authenticator{}
	auth.token.Store(azcore.AccessToken{Token: "test", ExpiresOn: time.Now().Add(time.Hour)})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = auth.getCurrentToken()
		}
	})
}
