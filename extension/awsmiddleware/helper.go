// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// TryConfigureSDKv1 is a helper function that will try to get the extension and configure the AWS SDKv1 handlers with it.
func TryConfigureSDKv1(logger *zap.Logger, extensions map[component.ID]component.Component, middlewareID ID, handlers *request.Handlers) {
	if c, err := GetConfigurer(extensions, middlewareID); err != nil {
		logger.Error("Unable to find AWS Middleware extension", zap.Error(err))
	} else if err = c.ConfigureSDKv1(handlers); err != nil {
		logger.Warn("Unable to configure middleware on AWS client", zap.Error(err))
	} else {
		logger.Debug("Configured middleware on AWS client", zap.String("middleware", middlewareID.String()))
	}
}

// TryConfigureSDKv2 is a helper function that will try to get the extension and configure the AWS SDKv2 config with it.
func TryConfigureSDKv2(logger *zap.Logger, extensions map[component.ID]component.Component, middlewareID ID, cfg *aws.Config) {
	if c, err := GetConfigurer(extensions, middlewareID); err != nil {
		logger.Error("Unable to find AWS Middleware extension", zap.Error(err))
	} else if err = c.ConfigureSDKv2(cfg); err != nil {
		logger.Warn("Unable to configure middleware on AWS client", zap.Error(err))
	} else {
		logger.Debug("Configured middleware on AWS client", zap.String("middleware", middlewareID.String()))
	}
}
