// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

import (
	"k8s.io/api/core/v1"
)

type podInfo struct {
	namespace string
	phase     v1.PodPhase
}
