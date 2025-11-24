// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package connection // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver/internal/connection"

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
)

// EstablishDeviceConnection creates a device connection using the provided DeviceConfig.
func EstablishDeviceConnection(ctx context.Context, device DeviceConfig, logger *zap.Logger) (*RPCClient, error) {
	authMethods, err := buildAuthMethods(device.Auth, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to build auth methods: %w", err)
	}

	sshConfig := &cryptossh.ClientConfig{
		User:            device.Auth.Username,
		Auth:            authMethods,
		HostKeyCallback: cryptossh.InsecureIgnoreHostKey(), // #nosec G106 - Insecure for lab/demo only
		Timeout:         10 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", device.Device.Host.IP, device.Device.Host.Port)

	conn, err := cryptossh.Dial("tcp", address, sshConfig)
	if err != nil {
		return nil, fmt.Errorf("SSH connection failed to %s: %w", address, err)
	}

	sshClient := &Client{
		Target:     address,
		Username:   device.Auth.Username,
		Connection: conn,
		Logger:     logger,
	}

	osType, err := sshClient.DetectOSType(ctx)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("OS detection failed: %w", err)
	}

	rpcClient := &RPCClient{
		SSHClient: sshClient,
		OSType:    osType,
		Logger:    logger,
	}

	return rpcClient, nil
}

// buildAuthMethods builds SSH authentication methods from the provided auth config.
// Supports both password and SSH key file authentication.
func buildAuthMethods(auth AuthConfig, logger *zap.Logger) ([]cryptossh.AuthMethod, error) {
	var authMethods []cryptossh.AuthMethod
	if auth.KeyFile != "" {
		keyAuth, err := publicKeyAuth(auth.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", auth.KeyFile, err)
		}
		authMethods = append(authMethods, keyAuth)
		logger.Debug("Using SSH key file authentication", zap.String("key_file", auth.KeyFile))
	}

	if auth.Password != "" {
		authMethods = append(authMethods, cryptossh.Password(string(auth.Password)))
		logger.Debug("Using password authentication")
	}

	if len(authMethods) == 0 {
		return nil, errors.New("no authentication method provided: either password or key_file is required")
	}

	return authMethods, nil
}

func publicKeyAuth(keyFile string) (cryptossh.AuthMethod, error) {
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("unable to read private key file: %w", err)
	}

	signer, err := cryptossh.ParsePrivateKey(key)
	if err != nil {
		var passphraseErr *cryptossh.PassphraseMissingError
		if errors.As(err, &passphraseErr) {
			return nil, fmt.Errorf("SSH key is encrypted but passphrase is not supported: %w", err)
		}
		return nil, fmt.Errorf("unable to parse private key: %w", err)
	}

	return cryptossh.PublicKeys(signer), nil
}
