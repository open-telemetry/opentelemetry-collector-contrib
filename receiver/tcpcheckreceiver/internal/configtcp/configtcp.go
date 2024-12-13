//// Copyright The OpenTelemetry Authors
//// SPDX-License-Identifier: Apache-2.0
//
//package configtcp // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver/internal/configtcp"
//
//import "C"
//import (
//	"context"
//	"errors"
//	"fmt"
//	"go.opentelemetry.io/collector/config/confignet"
//	"net"
//	"strconv"
//	"strings"
//)
//
//// Predefined error responses for configuration validation failures
//var (
//	errInvalidEndpoint = errors.New(`"Endpoint" must be in the form of <hostname>:<port>`)
//	errMissingTargets  = errors.New(`No targets specified`)
//	errConfigTCPCheck  = errors.New(`Invalid Config`)
//)
//
//// X this
//func (tcs *TCPClientSettings) ToClient() (*Client, error) {
//		TCPAddrConfig: confignet.TCPAddrConfig{
//			Endpoint: tcs.Endpoint,
//			DialerConfig: confignet.DialerConfig{
//				Timeout: tcs.Timeout,
//			},
//		},
//	}
//}
//
//func validatePort(port string) error {
//	portNum, err := strconv.Atoi(port)
//	if err != nil {
//		return fmt.Errorf("provided port is not a number: %s", port)
//	}
//	if portNum < 1 || portNum > 65535 {
//		return fmt.Errorf("provided port is out of valid range (1-65535): %d", portNum)
//	}
//	return nil
//}
//
//func (tcs *TCPClientSettings) ValidateTarget() error {
//	var err error
//
//	if tcs.Endpoint == "" {
//		return errMissingTargets
//	}
//
//	if strings.Contains(tcs.Endpoint, "://") {
//		return fmt.Errorf("endpoint contains a scheme, which is not allowed: %s", tcs.Endpoint)
//	}
//
//	_, port, parseErr := net.SplitHostPort(tcs.Endpoint)
//	if parseErr != nil {
//		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), parseErr)
//	}
//
//	portParseErr := validatePort(port)
//	if portParseErr != nil {
//		return fmt.Errorf("%s: %w", errInvalidEndpoint.Error(), portParseErr)
//	}
//
//	return err
//}
//
//// Client
//type Client struct {
//	Connection    net.Conn
//	Listener      net.Listener
//	TCPAddrConfig confignet.TCPAddrConfig
//}
//
//// Dial starts a TCP session.
//func (c *Client) Dial() (err error) {
//	c.Connection, err = c.TCPAddrConfig.Dial(context.Background())
//	if err != nil {
//		return err
//	}
//	return nil
//}
//
//// mock
//func (c *Client) Listen() (err error) {
//	c.Listener, err = c.TCPAddrConfig.Listen(context.Background())
//	if err != nil {
//		return err
//	}
//	return nil
//}
