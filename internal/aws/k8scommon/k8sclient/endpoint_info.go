// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

type endpointInfo struct {
	name       string //service name
	namespace  string //namespace name
	podKeyList []string
}
