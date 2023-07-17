// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/Azure/go-amqp"
	"go.uber.org/zap"
)

// inboundMessage is an alias for amqp.Message
type inboundMessage = amqp.Message

// messagingService abstracts out the AMQP transport capabilities for unit testing
type messagingService interface {
	dial(ctx context.Context) error
	close(ctx context.Context)
	receiveMessage(ctx context.Context) (*inboundMessage, error)
	accept(ctx context.Context, msg *inboundMessage) error
	failed(ctx context.Context, msg *inboundMessage) error
}

// messagingServiceFactory is a factory to create new messagingService instances
type messagingServiceFactory func() messagingService

// newAMQPMessagingServiceFactory creates a new messagingServiceFactory backed by AMQP
func newAMQPMessagingServiceFactory(cfg *Config, logger *zap.Logger) (messagingServiceFactory, error) {
	saslConnOption, authErr := toAMQPAuthentication(cfg)
	if authErr != nil {
		return nil, authErr
	}

	// Use the default load config for TLS. Note that in the case where "insecure" is true and no
	// ca file is provided, tlsConfig will be nil representing a plaintext connection.
	loadedTLSConfig, err := cfg.TLS.LoadTLSConfig()
	if err != nil {
		return nil, err
	}

	broker := cfg.Broker[0]
	// If the TLS config is nil, insecure is true and we should use amqp rather than amqps
	scheme := "amqp"
	if loadedTLSConfig != nil {
		scheme = "amqps"
	}
	amqpHostAddress := fmt.Sprintf("%s://%s", scheme, broker)

	connectConfig := &amqpConnectConfig{
		addr:       amqpHostAddress,
		tlsConfig:  loadedTLSConfig,
		saslConfig: saslConnOption,
	}

	receiverConfig := &amqpReceiverConfig{
		queue:       cfg.Queue,
		maxUnacked:  cfg.MaxUnacked,
		batchMaxAge: 1 * time.Second,
	}

	return func() messagingService {
		return &amqpMessagingService{
			connectConfig:  connectConfig,
			receiverConfig: receiverConfig,
			logger:         logger,
		}
	}, nil

}

type amqpConnectConfig struct {
	// conenct config
	addr       string
	saslConfig amqp.SASLType
	tlsConfig  *tls.Config
}

type amqpReceiverConfig struct {
	queue       string
	maxUnacked  int32
	batchMaxAge time.Duration
}

type amqpMessagingService struct {
	// factory fields
	connectConfig  *amqpConnectConfig
	receiverConfig *amqpReceiverConfig
	logger         *zap.Logger

	// runtime fields
	client   *amqp.Conn
	session  *amqp.Session
	receiver *amqp.Receiver
}

// dialFunc is abstracted out into a variable in order for substitutions
var dialFunc = amqp.Dial

// telemetryLinkName will be used to create the single receiver link in order to standardize the connection.
// Mainly useful for testing to mock amqp frames.
const telemetryLinkName = "rx"

func (m *amqpMessagingService) dial(ctx context.Context) (err error) {
	opts := &amqp.ConnOptions{}
	opts.SASLType = m.connectConfig.saslConfig
	if m.connectConfig.tlsConfig != nil {
		opts.TLSConfig = m.connectConfig.tlsConfig
	}
	m.logger.Debug("Dialing AMQP", zap.String("addr", m.connectConfig.addr))
	m.client, err = dialFunc(ctx, m.connectConfig.addr, opts)
	if err != nil {
		m.logger.Debug("Dial AMQP failure", zap.Error(err))
		return err
	}
	m.logger.Debug("Creating new AMQP Session")
	m.session, err = m.client.NewSession(ctx, &amqp.SessionOptions{})
	if err != nil {
		m.logger.Debug("Create AMQP Session failure", zap.Error(err))
		return err
	}
	m.logger.Debug("Creating new AMQP Receive Link", zap.String("source", m.receiverConfig.queue))
	m.receiver, err = m.session.NewReceiver(ctx, m.receiverConfig.queue, &amqp.ReceiverOptions{
		Credit: m.receiverConfig.maxUnacked,
		Name:   telemetryLinkName,
	})
	if err != nil {
		m.logger.Debug("Create AMQP Receiver Link failure", zap.Error(err))
		return err
	}
	return nil
}

func (m *amqpMessagingService) close(ctx context.Context) {
	if m.receiver != nil {
		m.logger.Debug("Closing AMQP Receiver")
		err := m.receiver.Close(ctx)
		if err != nil {
			m.logger.Debug("Receiver close failed", zap.Error(err))
		}
	}
	if m.session != nil {
		m.logger.Debug("Closing AMQP Session")
		err := m.session.Close(ctx)
		if err != nil {
			m.logger.Debug("Session closed failed", zap.Error(err))
		}
	}
	if m.client != nil {
		m.logger.Debug("Closing AMQP Client")
		err := m.client.Close()
		if err != nil {
			m.logger.Debug("Client closed failed", zap.Error(err))
		}
	}
}

func (m *amqpMessagingService) receiveMessage(ctx context.Context) (*inboundMessage, error) {
	return m.receiver.Receive(ctx, &amqp.ReceiveOptions{})
}

func (m *amqpMessagingService) accept(ctx context.Context, msg *inboundMessage) error {
	return m.receiver.AcceptMessage(ctx, msg)
}

func (m *amqpMessagingService) failed(ctx context.Context, msg *inboundMessage) error {
	return m.receiver.ModifyMessage(ctx, msg, &amqp.ModifyMessageOptions{
		DeliveryFailed:    true,
		UndeliverableHere: false,
		Annotations:       nil,
	})
}

// Allow for substitution in testing to assert correct data is passed to AMQP
// Due to the way that AMQP authentication is configured in Azure/amqp, we
// need to monkey substitute here since ConnSASL<auth> returns a function that
// acts on a private struct meaning we cannot meaningfully assert validity otherwise.
var (
	connSASLPlain    func(username, password string) amqp.SASLType                            = amqp.SASLTypePlain
	connSASLXOAUTH2  func(username, bearer string, maxFrameSizeOverride uint32) amqp.SASLType = amqp.SASLTypeXOAUTH2
	connSASLExternal func(resp string) amqp.SASLType                                          = amqp.SASLTypeExternal
)

// toAMQPAuthentication configures authentication in amqp.ConnOption slice
func toAMQPAuthentication(config *Config) (amqp.SASLType, error) {
	if config.Auth.PlainText != nil {
		plaintext := config.Auth.PlainText
		if plaintext.Password == "" || plaintext.Username == "" {
			return nil, errMissingPlainTextParams
		}
		return connSASLPlain(plaintext.Username, string(plaintext.Password)), nil
	}
	if config.Auth.XAuth2 != nil {
		xauth := config.Auth.XAuth2
		if xauth.Bearer == "" || xauth.Username == "" {
			return nil, errMissingXauth2Params
		}
		return connSASLXOAUTH2(xauth.Username, xauth.Bearer, saslMaxInitFrameSizeOverride), nil
	}
	if config.Auth.External != nil {
		return connSASLExternal(""), nil
	}
	return nil, errMissingAuthDetails
}
