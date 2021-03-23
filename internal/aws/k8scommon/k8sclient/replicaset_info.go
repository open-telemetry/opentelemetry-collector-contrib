// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

type replicaSetInfo struct {
	name   string
	owners []*replicaSetOwner
}

type replicaSetOwner struct {
	kind string
	name string
}
