// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver"

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	gojson "github.com/goccy/go-json"
	"go.opentelemetry.io/collector/client"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/receiver"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding"
)

const (
	bucketIDKey         = "bucketId"
	objectIDKey         = "objectId"
	eventTypeKey        = "eventType"
	eventObjectFinalize = "OBJECT_FINALIZE"

	bucketMetadataKey          = "bucket"
	objectMetadataKey          = "object"
	subscriptionMetadataKey    = "subscription"
	messageIDMetadataKey       = "message_id"
	deliveryAttemptMetadataKey = "delivery_attempt"
)

type pubSubPushReceiver struct {
	cfg           *Config
	settings      receiver.Settings
	storageClient *storage.Client

	server     *http.Server
	shutdownWG sync.WaitGroup

	nextLogs consumer.Logs
}

var _ receiver.Logs = (*pubSubPushReceiver)(nil)

func newPubSubPushReceiver(ctx context.Context, cfg *Config, set receiver.Settings) (*pubSubPushReceiver, error) {
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	return &pubSubPushReceiver{
		cfg:           cfg,
		settings:      set,
		storageClient: storageClient,
	}, nil
}

func (p *pubSubPushReceiver) registerLogsConsumer(lc consumer.Logs) {
	p.nextLogs = lc
}

func addHandlerFunc[T any](
	mux *http.ServeMux,
	endpoint string,
	unmarshal func([]byte) (T, error),
	consume func(context.Context, T) error,
	storageClient *storage.Client,
	includeMetadata bool,
	logger *zap.Logger,
) {
	mux.HandleFunc(endpoint, func(resp http.ResponseWriter, req *http.Request) {
		handlerCtx := req.Context()
		if err := handlePubSubPushRequest(
			handlerCtx,
			req.Body,
			unmarshal,
			consume,
			storageClient,
			includeMetadata,
		); err != nil {
			logger.Error("failed to handle Pub/Sub push request", zap.Error(err))
			code := http.StatusInternalServerError
			if consumererror.IsPermanent(err) {
				code = http.StatusBadRequest
			}
			http.Error(resp, err.Error(), code)
		}
	})
}

func (p *pubSubPushReceiver) Start(ctx context.Context, host component.Host) error {
	start := false
	mux := http.NewServeMux()
	// If this is in the logs pipeline, load the encoding extension for logs and
	// add a handler function in the logs' endpoint.
	if p.nextLogs != nil {
		start = true
		logsUnmarshaler, errLoad := loadEncodingExtension[encoding.LogsUnmarshalerExtension](
			host, p.cfg.Encoding, "logs",
		)
		if errLoad != nil {
			return fmt.Errorf("failed to load encoding extension: %w", errLoad)
		}
		addHandlerFunc(
			mux,
			"/",
			logsUnmarshaler.UnmarshalLogs,
			p.nextLogs.ConsumeLogs,
			p.storageClient,
			p.cfg.IncludeMetadata,
			p.settings.Logger,
		)
	}

	if !start {
		// should never happen
		return errors.New("failed to start Google Cloud Pub/Sub Push Receiver: no pipeline configured")
	}

	server, err := p.cfg.ToServer(ctx, host, p.settings.TelemetrySettings, mux)
	if err != nil {
		return err
	}
	p.server = server

	p.settings.Logger.Info("Starting HTTP server", zap.String("endpoint", p.cfg.Endpoint))
	lis, err := p.cfg.ToListener(ctx)
	if err != nil {
		return err
	}

	p.shutdownWG.Add(1)
	go func() {
		defer p.shutdownWG.Done()
		if errHTTP := p.server.Serve(lis); errHTTP != nil && !errors.Is(err, http.ErrServerClosed) {
			componentstatus.ReportStatus(host, componentstatus.NewFatalErrorEvent(err))
		}
	}()

	return nil
}

func (p *pubSubPushReceiver) Shutdown(ctx context.Context) error {
	var err error
	if p.server != nil {
		err = p.server.Shutdown(ctx)
	}
	p.shutdownWG.Wait()
	return err
}

// See: https://cloud.google.com/pubsub/docs/push
type pubSubPushRequest struct {
	Message         pubSubPushMessage `json:"message"`
	Subscription    string            `json:"subscription"`
	DeliveryAttempt int               `json:"deliveryAttempt"`
}

// See: https://cloud.google.com/pubsub/docs/reference/rest/v1/PubsubMessage
type pubSubPushMessage struct {
	Attributes  map[string]string `json:"attributes"`
	Data        []byte            `json:"data"` // go will automatically decode base64
	MessageID   string            `json:"messageId"`
	OrderingKey string            `json:"orderingKey,omitempty"`
	PublishTime time.Time         `json:"publishTime"`
}

// getFileContent retrieves file content from cloud storage notifications.
// Returns:
//   - (true, nil, nil) if the notification should be ignored (non-create event)
//   - (false, content, nil) if file content was successfully read
//   - (false, nil, error) if required attributes are missing or file read fails
//
// The function filters for OBJECT_FINALIZE events (new file creation) and
// ignores other storage events like deletion or metadata updates.
func getFileContent(
	ctx context.Context,
	attributes map[string]string,
	storageClient *storage.Client,
	extraMetadata map[string][]string,
) (bool, []byte, error) {
	bucket, exists := attributes[bucketIDKey]
	if !exists {
		return false, nil, nil
	}

	var object string
	if object, exists = attributes[objectIDKey]; !exists {
		return false, nil, consumererror.NewPermanent(fmt.Errorf("missing %s attribute", objectIDKey))
	}

	// See: https://cloud.google.com/storage/docs/pubsub-notifications#events
	var eventType string
	if eventType, exists = attributes[eventTypeKey]; !exists {
		return false, nil, consumererror.NewPermanent(fmt.Errorf("missing %s attribute", eventTypeKey))
	}
	// check that this notification is coming from a new file, otherwise ignore it
	if eventType != eventObjectFinalize {
		return true, nil, nil
	}

	reader, err := storageClient.Bucket(bucket).Object(object).NewReader(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("failed to create object reader: %w", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	// TODO https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/38780
	// Reading the whole file into memory is not good.
	body, err := io.ReadAll(reader)
	if err != nil {
		return false, nil, consumererror.NewPermanent(
			fmt.Errorf("failed to read file %s/%s content: %w", bucket, object, err),
		)
	}

	// enrich metadata
	if extraMetadata != nil {
		extraMetadata[bucketMetadataKey] = []string{bucket}
		extraMetadata[objectMetadataKey] = []string{object}
	}

	return false, body, nil
}

func handlePubSubPushRequest[T any](
	ctx context.Context,
	r io.Reader,
	unmarshal func([]byte) (T, error),
	consume func(context.Context, T) error,
	storageClient *storage.Client,
	includeMetadata bool,
) error {
	var request pubSubPushRequest
	if err := gojson.NewDecoder(r).Decode(&request); err != nil {
		return consumererror.NewPermanent(fmt.Errorf("failed to decode Pub/Sub request: %w", err))
	}

	var extraMetadata map[string][]string
	if includeMetadata {
		extraMetadata = make(map[string][]string)
	}

	ignore, unmarshalData, err := getFileContent(ctx, request.Message.Attributes, storageClient, extraMetadata)
	if err != nil {
		return err
	}
	if ignore {
		return nil
	}
	if unmarshalData == nil {
		// not coming from a storage notification, so we can
		// use the message data instead
		unmarshalData = request.Message.Data
	}

	unmarshalled, err := unmarshal(unmarshalData)
	if err != nil {
		return consumererror.NewPermanent(
			fmt.Errorf("failed to unmarshal message data from Pub/Sub request: %w", err),
		)
	}

	clCtx := ctx
	if extraMetadata != nil {
		if request.Subscription != "" {
			extraMetadata[subscriptionMetadataKey] = []string{request.Subscription}
		}
		if request.Message.MessageID != "" {
			extraMetadata[messageIDMetadataKey] = []string{request.Message.MessageID}
		}
		if request.DeliveryAttempt != 0 {
			extraMetadata[deliveryAttemptMetadataKey] = []string{fmt.Sprintf("%d", request.DeliveryAttempt)}
		}
		clCtx = client.NewContext(ctx, client.Info{
			Metadata: client.NewMetadata(extraMetadata),
		})
	}

	if err = consume(clCtx, unmarshalled); err != nil {
		consumeErr := fmt.Errorf("failed to consume unmarshalled request: %w", err)
		if consumererror.IsPermanent(err) {
			return consumererror.NewPermanent(consumeErr)
		}
		return consumererror.NewRetryableError(consumeErr)
	}
	return nil
}

func loadEncodingExtension[T any](host component.Host, encoding component.ID, signal string) (T, error) {
	var zero T
	ext, ok := host.GetExtensions()[encoding]
	if !ok {
		return zero, fmt.Errorf("extension %q not found", encoding.String())
	}
	unmarshaler, ok := ext.(T)
	if !ok {
		return zero, fmt.Errorf("extension %q is not a %s unmarshaler", encoding, signal)
	}
	return unmarshaler, nil
}
