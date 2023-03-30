// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"

	"github.com/docker/docker/api/server/router/session"
    "github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

var (
    errNilLogsConsumer = errors.New("Missing a logs consumer")
    errMissingEndpoint = errors.New("Missing a receiver endpoint")
)

type eventReceiver struct {
    settings    receiver.CreateSettings
    cfg         *Config
    logConsumer consumer.Logs
    server      *http.Server
    shutdownWG  sync.WaitGroup
    obsrecv     *obsreport.Receiver
    logger      *zap.Logger
}

// start function manages receiver startup tasks. part of the receiver.Logs interface.
func (er *eventReceiver) Start(ctx context.Context, host component.Host) error {
    // noop if not nil. if start has not been called before these values should be nil. 
    if er.server != nil && er.server.Handler != nil {
        return nil 
    }

    // create listener from config
    ln, err := er.cfg.HTTPServerSettings.ToListener()
    if err != nil {
        return err
    }

    // set up router
    router := gin.

    return nil
}

// stop function manages receiver shutdown tasks. part of the receiver.Logs interface.
func (er *eventReceiver) Shutdown(ctx context.Context) error {
    return nil
}

func newLogsReceiver(params receiver.CreateSettings, cfg Config, consumer consumer.Logs) (receiver.Logs, error) {
    if consumer == nil {
        return nil, errNilLogsConsumer
    }
    
    if cfg.Endpoint == "" {
        return nil, errMissingEndpoint
    }

    transport := "http"
    if cfg.TLSSetting != nil {
        transport = "https"
    }

    obsrecv, err := obsreport.NewReceiver(obsreport.ReceiverSettings{
        ReceiverID: params.ID,
        Transport: transport,
        ReceiverCreateSettings: params,
    })
    
    if err != nil {
        return nil, err 
    }

    readTimeout, err := time.ParseDuration(cfg.ReadTimeout+"ms")
    if err != nil {
        return nil, err 
    }

    writeTimeout, err := time.ParseDuration(cfg.WriteTimeout + "ms")
    if err != nil {
        return nil, err 
    }

    // create the server
    server := &http.Server{
        Addr: cfg.Endpoint,
        ReadHeaderTimeout: readTimeout,
        WriteTimeout: writeTimeout,
    }
    
    // create eventReceiver instance
    er := &eventReceiver{
        settings: params,
        cfg: &cfg,
        logConsumer: consumer,
        server: server,
        obsrecv: obsrecv,
        logger: params.Logger,
    }

    return er, nil
}
