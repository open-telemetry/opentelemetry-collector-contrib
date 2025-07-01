// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureauthextension

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/stretchr/testify/require"
)

func BenchmarkGetToken(b *testing.B) {
	m := mockTokenCredential{}
	m.On("GetToken").Return(azcore.AccessToken{Token: "test"}, nil)
	auth := authenticator{
		credential:  &m,
		tokensCache: make(map[string]azcore.AccessToken, 5),
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := auth.getTokenForHost(context.Background(), "test")
		require.NoError(b, err)
	}
}
