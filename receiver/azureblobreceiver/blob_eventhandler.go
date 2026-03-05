// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package azureblobreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver"

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

type blobEventHandler struct {
	blobClient          blobClient
	logsDataConsumer    logsDataConsumer
	tracesDataConsumer  tracesDataConsumer
	logsContainerName   string
	tracesContainerName string
	logger              *zap.Logger
	wg                  sync.WaitGroup
	cancelFunc          context.CancelFunc
	pollInterval        time.Duration
}

var _ eventHandler = (*blobEventHandler)(nil)

func (p *blobEventHandler) run(ctx context.Context) error {
	ctx, p.cancelFunc = context.WithCancel(ctx)

	p.wg.Add(1)
	go p.pollBlobs(ctx)

	return nil
}

func (p *blobEventHandler) pollBlobs(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	p.processContainers(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.processContainers(ctx)
		}
	}
}

func (p *blobEventHandler) processContainers(ctx context.Context) {
	if p.logsContainerName != "" && p.logsDataConsumer != nil {
		p.processContainer(ctx, p.logsContainerName, func(ctx context.Context, data []byte) error {
			return p.logsDataConsumer.consumeLogsJSON(ctx, data)
		})
	}

	if p.tracesContainerName != "" && p.tracesDataConsumer != nil {
		p.processContainer(ctx, p.tracesContainerName, func(ctx context.Context, data []byte) error {
			return p.tracesDataConsumer.consumeTracesJSON(ctx, data)
		})
	}
}

func (p *blobEventHandler) processContainer(ctx context.Context, containerName string, consume func(context.Context, []byte) error) {
	blobs, err := p.blobClient.listBlobs(ctx, containerName)
	if err != nil {
		p.logger.Error("failed to list blobs", zap.String("container", containerName), zap.Error(err))
		return
	}

	for _, blobName := range blobs {
		if ctx.Err() != nil {
			return
		}

		blobData, err := p.blobClient.readBlob(ctx, containerName, blobName)
		if err != nil {
			p.logger.Error("failed to read blob", zap.String("container", containerName), zap.String("blob", blobName), zap.Error(err))
			continue
		}

		if err := consume(ctx, blobData.Bytes()); err != nil {
			p.logger.Error("failed to consume blob data", zap.String("container", containerName), zap.String("blob", blobName), zap.Error(err))
		}
	}
}

func (p *blobEventHandler) close(_ context.Context) error {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	p.wg.Wait()
	return nil
}

func (p *blobEventHandler) setLogsDataConsumer(logsDataConsumer logsDataConsumer) {
	p.logsDataConsumer = logsDataConsumer
}

func (p *blobEventHandler) setTracesDataConsumer(tracesDataConsumer tracesDataConsumer) {
	p.tracesDataConsumer = tracesDataConsumer
}

func newBlobEventHandler(logsContainerName, tracesContainerName string, blobClient blobClient, logger *zap.Logger) *blobEventHandler {
	return &blobEventHandler{
		blobClient:          blobClient,
		logsContainerName:   logsContainerName,
		tracesContainerName: tracesContainerName,
		logger:              logger,
		pollInterval:        10 * time.Second,
	}
}
