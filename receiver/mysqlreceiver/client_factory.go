// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mysqlreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver"

import (
	"context"
	"crypto/tls"
	"database/sql"
	"database/sql/driver"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"go.opentelemetry.io/collector/config/confignet"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/dbauth"
)

const mysqlCustomTLSConfigName = "custom"

type mySQLClientFactory interface {
	connect(ctx context.Context) (client, error)
	setCredentialProvider(provider dbauth.Provider)
	close() error
}

func newClientFactory(cfg *Config) (mySQLClientFactory, error) {
	base, err := buildMySQLConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &defaultClientFactory{
		baseConfig: base,
		stmtEvents: cfg.StatementEvents,
	}, nil
}

type defaultClientFactory struct {
	baseConfig mySQLConfig
	stmtEvents StatementEventsConfig
}

func (d *defaultClientFactory) setCredentialProvider(provider dbauth.Provider) {
	d.baseConfig.credentialProvider = provider
}

func (d *defaultClientFactory) connect(ctx context.Context) (client, error) {
	db, err := getDB(ctx, d.baseConfig)
	if err != nil {
		return nil, err
	}
	c := newMySQLClientFromDB(db, d.stmtEvents)
	c.populateDBVersion()
	return c, nil
}

func (*defaultClientFactory) close() error {
	return nil
}

func buildMySQLConfig(cfg *Config) (mySQLConfig, error) {
	mc := mySQLConfig{
		username:             cfg.Username,
		password:             string(cfg.Password),
		database:             cfg.Database,
		allowNativePasswords: cfg.AllowNativePasswords,
		address:              cfg.AddrConfig,
	}
	tlsCfg, err := cfg.TLS.LoadTLSConfig(context.Background())
	if err != nil {
		return mc, err
	}
	if tlsCfg != nil {
		if err := registerMySQLTLSConfig(tlsCfg); err != nil {
			return mc, err
		}
		mc.tlsConfigName = mysqlCustomTLSConfigName
	}
	return mc, nil
}

func registerMySQLTLSConfig(tlsCfg *tls.Config) error {
	err := mysql.RegisterTLSConfig(mysqlCustomTLSConfigName, tlsCfg)
	if err != nil && !isMySQLTLSConfigAlreadyRegistered(err) {
		return err
	}
	return nil
}

func isMySQLTLSConfigAlreadyRegistered(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already registered")
}

type mySQLConfig struct {
	username             string
	password             string
	database             string
	allowNativePasswords bool
	tlsConfigName        string
	address              confignet.AddrConfig
	credentialProvider   dbauth.Provider
}

func (c mySQLConfig) driverConfig(ctx context.Context) (*mysql.Config, error) {
	username, password := c.username, c.password
	if c.credentialProvider != nil {
		cred, credErr := c.credentialProvider.GetCredential(ctx, dbauth.Request{
			Endpoint: c.address.Endpoint,
			Username: c.username,
		})
		if credErr != nil {
			return nil, errors.New("resolve credential: " + credErr.Error())
		}
		if cred == nil {
			return nil, errors.New("resolve credential: provider returned a nil credential")
		}
		password = cred.Secret
		if cred.Username != nil {
			username = *cred.Username
		}
	}

	return &mysql.Config{
		User:                 username,
		Passwd:               password,
		Net:                  string(c.address.Transport),
		Addr:                 c.address.Endpoint,
		DBName:               c.database,
		AllowNativePasswords: c.allowNativePasswords,
		TLSConfig:            c.tlsConfigName,
	}, nil
}

func (c mySQLConfig) ConnectionString(ctx context.Context) (string, error) {
	driverConf, err := c.driverConfig(ctx)
	if err != nil {
		return "", err
	}
	return driverConf.FormatDSN(), nil
}

func getDB(ctx context.Context, cfg mySQLConfig) (*sql.DB, error) {
	if cfg.credentialProvider != nil {
		return sql.OpenDB(&credentialConnector{cfg: cfg}), nil
	}
	driverConf, err := cfg.driverConfig(ctx)
	if err != nil {
		return nil, err
	}
	connector, err := mysql.NewConnector(driverConf)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

type credentialConnector struct {
	cfg mySQLConfig
}

func (c *credentialConnector) Connect(ctx context.Context) (driver.Conn, error) {
	driverConf, err := c.cfg.driverConfig(ctx)
	if err != nil {
		return nil, err
	}
	connector, err := mysql.NewConnector(driverConf)
	if err != nil {
		return nil, err
	}
	return connector.Connect(ctx)
}

func (*credentialConnector) Driver() driver.Driver { return &mysql.MySQLDriver{} }
