// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator"

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unicode"

	"github.com/expr-lang/expr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer"
)

func evalConfigExpression(expression string, env observer.EndpointEnv) (any, error) {
	program, err := expr.Compile(
		expression,
		expr.Function("joinHostPort", joinHostPort, new(func(any, any) string)),
	)
	if err != nil {
		return nil, err
	}
	return expr.Run(program, env)
}

func joinHostPort(params ...any) (any, error) {
	if len(params) != 2 {
		return nil, fmt.Errorf("joinHostPort expects 2 arguments, got %d", len(params))
	}

	host, err := bareHost(params[0])
	if err != nil {
		return nil, err
	}
	port, err := numericPort(params[1])
	if err != nil {
		return nil, err
	}

	return net.JoinHostPort(host, port), nil
}

func bareHost(value any) (string, error) {
	host, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("host must be a string, got %T", value)
	}
	if host == "" {
		return "", errors.New("host must not be empty")
	}
	// Reject URL, userinfo, bracket, and port syntax while allowing bare IPv6 literals.
	if strings.IndexFunc(host, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}) >= 0 || strings.ContainsAny(host, "[]/?#@\\'\"`") {
		return "", errors.New("host must be a bare hostname or IP address")
	}
	if strings.Contains(host, ":") {
		if _, err := netip.ParseAddr(host); err != nil {
			return "", errors.New("host must be a bare hostname or IP address")
		}
	}
	return host, nil
}

func numericPort(value any) (string, error) {
	var port uint64
	switch value := value.(type) {
	case string:
		var err error
		port, err = strconv.ParseUint(value, 10, 16)
		if err != nil {
			return "", errors.New("port must be a decimal number between 1 and 65535")
		}
	case int:
		if value < 1 || value > 65535 {
			return "", errors.New("port must be between 1 and 65535")
		}
		port = uint64(value)
	case uint16:
		port = uint64(value)
	default:
		return "", fmt.Errorf("port must be a string or integer, got %T", value)
	}
	if port == 0 {
		return "", errors.New("port must be between 1 and 65535")
	}
	return strconv.FormatUint(port, 10), nil
}
