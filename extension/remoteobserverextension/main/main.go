// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/remoteobserverextension/main"

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/remoteobserverextension"
)

func main() {
	f := remoteobserverextension.NewFactory()
	cs := extensiontest.NewNopCreateSettings()
	var err error
	cs.Logger, err = zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	ext, err := f.CreateExtension(context.Background(), cs, f.CreateDefaultConfig())
	if err != nil {
		cs.Logger.Fatal(err.Error())
	}
	err = ext.Start(context.Background(), componenttest.NewNopHost())
	if err != nil {
		cs.Logger.Fatal(err.Error())
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	waitCh := make(chan struct{})
	go func() {
		<-c
		err = ext.Shutdown(context.Background())
		if err != nil {
			cs.Logger.Fatal(err.Error())
		}
		close(waitCh)
	}()

	<-waitCh
}
