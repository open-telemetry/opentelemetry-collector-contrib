// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package oauth2clientcredentialsauthextension implements `configauth.ClientAuthenticator`
// This extension provides OAuth2 Client Credentials flow authenticator for HTTP and gRPC based exporters.
// The extension fetches and refreshes the token after expiry
// For further details about OAuth2 Client Credentials flow refer https://datatracker.ietf.org/doc/html/rfc6749#section-4.4
package oauth2clientauthextension
