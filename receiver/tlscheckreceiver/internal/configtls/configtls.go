package configtls

import (
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtls"
)

var network = "tcp"

type TLSCertsClientSettings struct {
	// Endpoint is always required
	Endpoint string `mapstructure:"endpoint"`

	// TLSSetting struct exposes TLS client configuration.
	TLSSetting configtls.TLSClientSetting `mapstructure:"tls"`

	// Timeout parameter configures `tls.Client.Timeout`.
	Timeout time.Duration `mapstructure:"timeout"`

	// file path to the local_cert_path
	LocalCertPath string `mapstructure:"local_cert_path"`
}

type Client struct {
	*tls.Conn
	*tls.Config
	DialWithDialerFunc func(dialer *net.Dialer, network string, addr string, config *tls.Config) (*tls.Conn, error)
}

type TLSClient struct {
	server string
	*tls.Config
}

func (c *Client) DialWithDialer(address string) (err error) {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}
	c.Conn, err = c.DialWithDialerFunc(dialer, network, address, c.Config)
	if err != nil {
		return err
	}
	defer c.Conn.Close()

	err = c.Conn.Handshake()
	if err != nil {
		return fmt.Errorf("failed to handshake: %v", err)
	}

	return nil
}

func (t *TLSCertsClientSettings) ToClient(host component.Host, settings component.TelemetrySettings) (*Client, error) {
	return &Client{
		Config: &tls.Config{
			InsecureSkipVerify: t.TLSSetting.InsecureSkipVerify,
		},
		DialWithDialerFunc: tls.DialWithDialer,
	}, nil
}
