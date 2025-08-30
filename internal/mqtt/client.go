// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mqtt // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/mqtt"

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

type MqttClient interface {
	DialConfig(config DialConfig) (Connection, error)
}

type Connection interface {
	ReconnectIfUnhealthy() error
	IsConnected() bool
	Publish(ctx context.Context, topic string, qos byte, retain bool, payload []byte) error
	Close() error
}

type connectionHolder struct {
	config           DialConfig
	client           mqtt.Client
	logger           *zap.Logger
	connLock         *sync.Mutex
	connectionErrors chan error
}

type DialConfig struct {
	BrokerURLs              []string
	ClientID                string
	Username                string
	Password                string
	ConnectTimeout          time.Duration
	KeepAlive               time.Duration
	TLS                     *tls.Config
	AutoReconnect           bool
	ConnectRetry            bool
	ConnectRetryInterval    time.Duration
	MaxReconnectInterval    time.Duration
	PingTimeout             time.Duration
	PublishConfirmationTimeout time.Duration
}

func NewMqttClient(logger *zap.Logger) MqttClient {
	return &client{logger: logger}
}

type client struct {
	logger *zap.Logger
}

func (c *client) DialConfig(config DialConfig) (Connection, error) {
	ch := &connectionHolder{
		config:           config,
		logger:           c.logger,
		connLock:         &sync.Mutex{},
		connectionErrors: make(chan error, 1),
	}

	ch.connLock.Lock()
	defer ch.connLock.Unlock()

	err := ch.connect()
	return ch, err
}

func (c *connectionHolder) ReconnectIfUnhealthy() error {
	c.connLock.Lock()
	defer c.connLock.Unlock()

	hasConnectionError := false
	select {
	case err := <-c.connectionErrors:
		hasConnectionError = true
		c.logger.Info("Received connection error, will retry restoring unhealthy connection", zap.Error(err))
	default:
		break
	}

	if hasConnectionError || !c.isConnected() {
		if c.isConnected() {
			c.client.Disconnect(250)
		}

		if err := c.connect(); err != nil {
			return errors.Join(errors.New("failed attempt at restoring unhealthy connection"), err)
		}
		c.logger.Info("Successfully restored unhealthy MQTT connection")
	}

	return nil
}

func (c *connectionHolder) connect() error {
	c.logger.Debug("Connecting to MQTT broker")

	// Parse broker URLs
	var servers []*url.URL
	for _, brokerURL := range c.config.BrokerURLs {
		parsedURL, err := url.Parse(brokerURL)
		if err != nil {
			return fmt.Errorf("invalid broker URL %s: %w", brokerURL, err)
		}
		servers = append(servers, parsedURL)
	}

	// Set up MQTT client options
	opts := mqtt.NewClientOptions()
	for _, server := range servers {
		opts.AddBroker(server.String())
	}
	
	opts.SetClientID(c.config.ClientID)
	opts.SetUsername(c.config.Username)
	opts.SetPassword(c.config.Password)
	opts.SetConnectTimeout(c.config.ConnectTimeout)
	opts.SetKeepAlive(c.config.KeepAlive)
	
	if c.config.TLS != nil {
		opts.SetTLSConfig(c.config.TLS)
	}

	// Connection resilience settings
	opts.SetAutoReconnect(c.config.AutoReconnect)
	opts.SetConnectRetry(c.config.ConnectRetry)
	opts.SetConnectRetryInterval(c.config.ConnectRetryInterval)
	opts.SetMaxReconnectInterval(c.config.MaxReconnectInterval)
	opts.SetPingTimeout(c.config.PingTimeout)

	// Log lifecycle events
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		c.logger.Info("MQTT connected")
	})
	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		c.logger.Warn("MQTT connection lost", zap.Error(err))
		select {
		case c.connectionErrors <- err:
		default:
		}
	})

	// Create and connect MQTT client
	client := mqtt.NewClient(opts)
	c.client = client

	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}

	c.logger.Info("Connected to MQTT broker")
	return nil
}

func (c *connectionHolder) Close() error {
	if c.isConnected() {
		c.client.Disconnect(250)
	}
	return nil
}

func (c *connectionHolder) isConnected() bool {
	return c.client != nil && c.client.IsConnected()
}

func (c *connectionHolder) IsConnected() bool {
	c.connLock.Lock()
	defer c.connLock.Unlock()
	return c.isConnected()
}

func (c *connectionHolder) Publish(ctx context.Context, topic string, qos byte, retain bool, payload []byte) error {
	if !c.isConnected() {
		return errors.New("MQTT client is not connected")
	}

	// Publish message with QoS and retain settings
	token := c.client.Publish(topic, qos, retain, payload)

	// Wait for the token to complete with timeout
	select {
	case <-token.Done():
		if token.Error() != nil {
			c.logger.Error("Error publishing message", zap.Error(token.Error()))
			return token.Error()
		}
		c.logger.Debug("Message published successfully")
		return nil
	case <-time.After(c.config.PublishConfirmationTimeout):
		c.logger.Warn("Timeout waiting for publish confirmation", zap.Duration("timeout", c.config.PublishConfirmationTimeout))
		return fmt.Errorf("timeout waiting for publish confirmation after %s", c.config.PublishConfirmationTimeout)
	case <-ctx.Done():
		return ctx.Err()
	}
}
