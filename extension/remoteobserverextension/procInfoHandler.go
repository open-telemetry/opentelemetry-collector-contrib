// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remoteobserverextension

import (
	"net/http"
)

type procInfoHandler struct {
	wpr *wsProcRegistry
}

func (p procInfoHandler) ServeHTTP(resp http.ResponseWriter, _ *http.Request) {
	bytes, _ := p.wpr.toJSON()
	_, _ = resp.Write(bytes)
}
