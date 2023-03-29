// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package webhookeventreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/docker/docker/api/server/router/session"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"
)

var (
    errNilLogsConsumer = errors.New("Missing a logs consumer")
    errMissingEndpoint     = errors.New("Missing a receiver endpoint")
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

func (er *eventReceiver) Start(ctx context.Context, host component.Host) error {
    return nil
}

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
