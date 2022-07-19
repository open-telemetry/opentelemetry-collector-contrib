// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver"

import (
	"context"
	"fmt"

	"github.com/Azure/go-amqp"
	"go.uber.org/zap"
)

// inboundMessage is an alias for amqp.Message
type inboundMessage = amqp.Message

// messagingService abstracts out the AMQP transport capabilities for unit testing
type messagingService interface {
	dial() error
	close(ctx context.Context)
	receiveMessage(ctx context.Context) (*inboundMessage, error)
	ack(ctx context.Context, msg *inboundMessage) error
	nack(ctx context.Context, msg *inboundMessage) error
}

// messagingServiceFactory is a factory to create new messagingService instances
type messagingServiceFactory func() messagingService

// connTLSConfig abstracts out amqp.ConnTLSConfig in order for substitution in tests
var connTLSConfig = amqp.ConnTLSConfig

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

	var tlsConfig amqp.ConnOption

	broker := cfg.Broker[0]
	// If the TLS config is nil, insecure is true and we should use amqp rather than amqps
	scheme := "amqp"
	if loadedTLSConfig != nil {
		scheme = "amqps"
		tlsConfig = connTLSConfig(loadedTLSConfig)
	}
	amqpHostAddress := fmt.Sprintf("%s://%s", scheme, broker)

	connectConfig := &amqpConnectConfig{
		addr:       amqpHostAddress,
		tlsConfig:  tlsConfig,
		saslConfig: saslConnOption,
	}

	receiverConfig := &amqpReceiverConfig{
		queue:      cfg.Queue,
		maxUnacked: cfg.MaxUnacked,
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
	saslConfig amqp.ConnOption
	tlsConfig  amqp.ConnOption
}

type amqpReceiverConfig struct {
	queue      string
	maxUnacked uint32
}

type amqpMessagingService struct {
	// factory fields
	connectConfig  *amqpConnectConfig
	receiverConfig *amqpReceiverConfig
	logger         *zap.Logger

	// runtime fields
	client   *amqp.Client
	session  *amqp.Session
	receiver *amqp.Receiver
}

// dialFunc is abstracted out into a variable in order for substitutions
var dialFunc = amqp.Dial

// telemetryLinkName will be used to create the single receiver link in order to standardize the connection.
// Mainly useful for testing to mock amqp frames.
const telemetryLinkName = "rx"

func (m *amqpMessagingService) dial() (err error) {
	opts := []amqp.ConnOption{m.connectConfig.saslConfig}
	if m.connectConfig.tlsConfig != nil {
		opts = append(opts, m.connectConfig.tlsConfig)
	}
	m.logger.Debug("Dialing AMQP", zap.String("addr", m.connectConfig.addr))
	m.client, err = dialFunc(m.connectConfig.addr, opts...)
	if err != nil {
		m.logger.Debug("Dial AMQP failure", zap.Error(err))
		return err
	}
	m.logger.Debug("Creating new AMQP Session")
	m.session, err = m.client.NewSession()
	if err != nil {
		m.logger.Debug("Create AMQP Session failure", zap.Error(err))
		return err
	}
	m.logger.Debug("Creating new AMQP Receive Link", zap.String("source", m.receiverConfig.queue))
	m.receiver, err = m.session.NewReceiver(
		amqp.LinkSourceAddress(m.receiverConfig.queue),
		amqp.LinkCredit(m.receiverConfig.maxUnacked),
		amqp.LinkName(telemetryLinkName),
	)
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
	return m.receiver.Receive(ctx)
}

func (m *amqpMessagingService) ack(ctx context.Context, msg *inboundMessage) error {
	return m.receiver.AcceptMessage(ctx, msg)
}

func (m *amqpMessagingService) nack(ctx context.Context, msg *inboundMessage) error {
	return m.receiver.RejectMessage(ctx, msg, nil)
}

// Allow for substitution in testing to assert correct data is passed to AMQP
// Due to the way that AMQP authentication is configured in Azure/amqp, we
// need to monkey substitute here since ConnSASL<auth> returns a function that
// acts on a private struct meaning we cannot meaningfully assert validity otherwise.
var (
	connSASLPlain    func(username, password string) amqp.ConnOption                            = amqp.ConnSASLPlain
	connSASLXOAUTH2  func(username, bearer string, maxFrameSizeOverride uint32) amqp.ConnOption = amqp.ConnSASLXOAUTH2
	connSASLExternal func(resp string) amqp.ConnOption                                          = amqp.ConnSASLExternal
)

// toAMQPAuthentication configures authentication in amqp.ConnOption slice
func toAMQPAuthentication(config *Config) (amqp.ConnOption, error) {
	if config.Auth.PlainText != nil {
		plaintext := config.Auth.PlainText
		if plaintext.Password == "" || plaintext.Username == "" {
			return nil, errMissingPlainTextParams
		}
		return connSASLPlain(plaintext.Username, plaintext.Password), nil
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
