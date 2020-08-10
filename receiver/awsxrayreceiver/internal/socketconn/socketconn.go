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

package socketconn

// SocketConn is an interface for socket connection.
type SocketConn interface {
	// Reads a packet from the connection, copying the payload into b. It returns number of bytes copied.
	Read(b []byte) (int, error)

	// Closes the connection.
	Close() error
}
