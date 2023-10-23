// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/request"
)

type SDKVersion interface {
	unused()
}

type sdkVersion1 struct {
	SDKVersion
	handlers *request.Handlers
}

// SDKv1 takes in AWS SDKv1 client request handlers.
func SDKv1(handlers *request.Handlers) SDKVersion {
	return sdkVersion1{handlers: handlers}
}

type sdkVersion2 struct {
	SDKVersion
	cfg *aws.Config
}

// SDKv2 takes in an AWS SDKv2 config.
func SDKv2(cfg *aws.Config) SDKVersion {
	return sdkVersion2{cfg: cfg}
}
