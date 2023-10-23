// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsmiddleware // import "github.com/amazon-contributing/opentelemetry-collector-contrib/extension/awsmiddleware"

import (
	"go.opentelemetry.io/collector/component"
	"go.uber.org/zap"
)

// TryConfigure is a helper function that will try to get the extension and configure the provided AWS SDK with it.
func TryConfigure(logger *zap.Logger, host component.Host, middlewareID component.ID, sdkVersion SDKVersion) {
	if c, err := GetConfigurer(host.GetExtensions(), middlewareID); err != nil {
		logger.Error("Unable to find AWS Middleware extension", zap.Error(err))
	} else if err = c.Configure(sdkVersion); err != nil {
		logger.Warn("Unable to configure middleware on AWS client", zap.Error(err))
	} else {
		logger.Debug("Configured middleware on AWS client", zap.String("middleware", middlewareID.String()))
	}
}
