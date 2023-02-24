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

package rest

/*
	streaming : Hot buckets that need to be rolled or have their size committed.
	data_safety : Buckets without at least two rawdata copies.
	generation : Buckets without a primary copy.
	replication_factor : Buckets without replication factor number of copies.
	search_factor : Buckets without search factor number of copies.
	checksum_sync : Level for syncing a bucket's delete files across all peers that have this bucket. Syncing is determined based on the checksum of all of the delete files.
*/

type FixupLevel int8

const (
	_ FixupLevel = iota
	// FixupLevelStreaming - Hot buckets that need to be rolled or have their size committed.
	FixupLevelStreaming
	// FixupLevelDataSafety - Buckets without at least two rawdata copies.
	FixupLevelDataSafety
	// FixupLevelGeneration - Buckets without a primary copy.
	FixupLevelGeneration
	// FixupLevelReplicationFactor - Buckets without replication factor number of copies.
	FixupLevelReplicationFactor
	// FixupLevelSearchFactor - Buckets without search factor number of copies.
	FixupLevelSearchFactor
	// FixupLevelChecksumSync - Level for syncing a bucket's delete files across all peers that have this bucket. Syncing is determined based on the checksum of all of the delete files.
	FixupLevelChecksumSync
)

func (fl FixupLevel) String() string {
	switch fl {
	case FixupLevelStreaming:
		return "streaming"
	case FixupLevelDataSafety:
		return "data_safety"
	case FixupLevelGeneration:
		return "generation"
	case FixupLevelReplicationFactor:
		return "replication_factor"
	case FixupLevelSearchFactor:
		return "search_factor"
	case FixupLevelChecksumSync:
		return "checksum_sync"
	}
	return "unknown"
}
