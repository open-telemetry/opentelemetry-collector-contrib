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

package model

import (
	"bytes"
	"testing"
)

func TestCollectionPeriodProto(t *testing.T) {
	var period CollectionPeriod = "MIN_5"
	p, err := period.Proto()

	if err != nil || p != 300 {
		t.Errorf("improper conversion to proto")
	}
}

func TestCollectionPeriodIntLiteralProto(t *testing.T) {
	var period CollectionPeriod = "2"
	p, err := period.Proto()

	if err != nil || p != 2 {
		t.Errorf("improper conversion to proto")
	}
}

func TestCollectionPeriodNotFoundProto(t *testing.T) {
	var period CollectionPeriod = "DAY_69105"
	_, err := period.Proto()

	if err == nil {
		t.Errorf("conversion should have failed with period: %v", period)
	}
}

func TestCollectionPeriodHash(t *testing.T) {
	var configA CollectionPeriod = "NONE"
	var configB CollectionPeriod = "DAY_69105"
	var configC CollectionPeriod = "SEC_1"

	if !bytes.Equal(configA.Hash(), configB.Hash()) {
		t.Errorf("identical configs with different hashes")
	}

	if bytes.Equal(configA.Hash(), configC.Hash()) {
		t.Errorf("different configs with identical hashes")
	}
}
