// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package snmptrapreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmptrapreceiver"

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

// ListenConfig is the UDP listener + USM settings shared by the OTel receiver.
type ListenConfig struct {
	ListenAddress    string
	Communities      []string
	IncludeCommunity bool
	DropUndefined    bool
	MIBPaths         []string
	V3               *V3Config
}

// CommunityAllowed reports whether a v1/v2c community is accepted.
// An empty allowlist accepts every community.
func CommunityAllowed(allow []string, got string) bool {
	if len(allow) == 0 {
		return true
	}
	for _, c := range allow {
		if c == got {
			return true
		}
	}
	return false
}

// ListenChanged reports whether the UDP bind / auth / MIB tree changed.
func ListenChanged(old, new ListenConfig) bool {
	if old.ListenAddress != new.ListenAddress {
		return true
	}
	if !slices.Equal(old.Communities, new.Communities) {
		return true
	}
	if !slices.Equal(old.MIBPaths, new.MIBPaths) {
		return true
	}
	return v3Changed(old.V3, new.V3)
}

func v3Changed(a, b *V3Config) bool {
	if a == nil && b == nil {
		return false
	}
	if a == nil || b == nil {
		return true
	}
	return a.User != b.User || a.SecurityLevel != b.SecurityLevel ||
		a.AuthProtocol != b.AuthProtocol || a.PrivProtocol != b.PrivProtocol ||
		a.AuthPassword != b.AuthPassword || a.PrivPassword != b.PrivPassword
}

// GoSNMPParams builds gosnmp listener parameters. v3 is optional; omit it
// for v1/v2c. When V3 is set the listener is v3-only.
func GoSNMPParams(cfg ListenConfig, log *slog.Logger) (*gosnmp.GoSNMP, error) {
	params := &gosnmp.GoSNMP{
		Port:               162,
		Transport:          "udp",
		Version:            gosnmp.Version2c,
		Timeout:            time.Second,
		Retries:            0,
		ExponentialTimeout: true,
		MaxOids:            60,
		Logger:             gosnmp.NewLogger(snmpLogger{log: log}),
	}
	if cfg.V3 == nil {
		return params, nil
	}

	params.Version = gosnmp.Version3
	params.SecurityModel = gosnmp.UserSecurityModel
	flags, err := mapMsgFlags(cfg.V3.SecurityLevel)
	if err != nil {
		return nil, err
	}
	params.MsgFlags = flags

	auth, err := mapAuthProtocol(cfg.V3.AuthProtocol)
	if err != nil {
		return nil, err
	}
	priv, err := mapPrivProtocol(cfg.V3.PrivProtocol)
	if err != nil {
		return nil, err
	}
	params.SecurityParameters = &gosnmp.UsmSecurityParameters{
		UserName:                 cfg.V3.User,
		AuthenticationProtocol:   auth,
		AuthenticationPassphrase: cfg.V3.AuthPassword,
		PrivacyProtocol:          priv,
		PrivacyPassphrase:        cfg.V3.PrivPassword,
	}
	return params, nil
}

func mapMsgFlags(level string) (gosnmp.SnmpV3MsgFlags, error) {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "", "noauthnopriv":
		return gosnmp.NoAuthNoPriv, nil
	case "authnopriv":
		return gosnmp.AuthNoPriv, nil
	case "authpriv":
		return gosnmp.AuthPriv, nil
	default:
		return 0, fmt.Errorf("unknown security_level %q", level)
	}
}

func mapAuthProtocol(p string) (gosnmp.SnmpV3AuthProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "", "none", "noauth":
		return gosnmp.NoAuth, nil
	case "md5":
		return gosnmp.MD5, nil
	case "sha":
		return gosnmp.SHA, nil
	case "sha224":
		return gosnmp.SHA224, nil
	case "sha256":
		return gosnmp.SHA256, nil
	case "sha384":
		return gosnmp.SHA384, nil
	case "sha512":
		return gosnmp.SHA512, nil
	default:
		return 0, fmt.Errorf("unknown auth_protocol %q", p)
	}
}

func mapPrivProtocol(p string) (gosnmp.SnmpV3PrivProtocol, error) {
	switch strings.ToLower(strings.TrimSpace(p)) {
	case "", "none", "nopriv":
		return gosnmp.NoPriv, nil
	case "des":
		return gosnmp.DES, nil
	case "aes":
		return gosnmp.AES, nil
	case "aes192":
		return gosnmp.AES192, nil
	case "aes192c":
		return gosnmp.AES192C, nil
	case "aes256":
		return gosnmp.AES256, nil
	case "aes256c":
		return gosnmp.AES256C, nil
	default:
		return 0, fmt.Errorf("unknown priv_protocol %q", p)
	}
}

type snmpLogger struct {
	log *slog.Logger
}

func (s snmpLogger) Print(v ...interface{}) {
	if s.log == nil {
		return
	}
	s.log.Debug(fmt.Sprint(v...))
}

func (s snmpLogger) Printf(format string, v ...interface{}) {
	if s.log == nil {
		return
	}
	s.log.Debug(fmt.Sprintf(format, v...))
}

// PDUKind is "inform" or "trap" for metrics.
func PDUKind(t gosnmp.PDUType) string {
	if t == gosnmp.InformRequest {
		return "inform"
	}
	return "trap"
}
