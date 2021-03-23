// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: MIT

package k8sclient

type jobInfo struct {
	name   string
	owners []*jobOwner
}

type jobOwner struct {
	kind string
	name string
}
