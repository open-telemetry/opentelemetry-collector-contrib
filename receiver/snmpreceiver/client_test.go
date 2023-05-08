// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package snmpreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver"

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/receiver/scrapererror"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver/internal/mocks"
)

func TestNewClient(t *testing.T) {
	testCase := []struct {
		desc        string
		cfg         *Config
		host        component.Host
		settings    component.TelemetrySettings
		logger      *zap.Logger
		expectError error
	}{
		{
			desc: "Valid v2c configuration",
			cfg: &Config{
				Version:   "v2c",
				Endpoint:  "udp://localhost:161",
				Community: "public",
			},
			host:        componenttest.NewNopHost(),
			settings:    componenttest.NewNopTelemetrySettings(),
			logger:      zap.NewNop(),
			expectError: nil,
		},
		{
			desc: "Valid v3 configuration",
			cfg: &Config{
				Version:         "v3",
				Endpoint:        "tcp://localhost:161",
				User:            "user",
				SecurityLevel:   "auth_priv",
				AuthType:        "MD5",
				AuthPassword:    "authpass",
				PrivacyType:     "DES",
				PrivacyPassword: "privacypass",
			},
			host:        componenttest.NewNopHost(),
			settings:    componenttest.NewNopTelemetrySettings(),
			logger:      zap.NewNop(),
			expectError: nil,
		},
	}

	for _, tc := range testCase {
		t.Run(tc.desc, func(t *testing.T) {
			ac, err := newClient(tc.cfg, tc.logger)
			if tc.expectError != nil {
				require.Nil(t, ac)
				require.Contains(t, err.Error(), tc.expectError.Error())
			} else {
				require.NoError(t, err)

				actualClient, ok := ac.(*snmpClient)
				require.True(t, ok)

				compareConfigToClient(t, actualClient, tc.cfg)
			}
		})
	}
}

func compareConfigToClient(t *testing.T, client *snmpClient, cfg *Config) {
	t.Helper()

	require.True(t, strings.Contains(cfg.Endpoint, client.client.GetTarget()))
	require.True(t, strings.Contains(cfg.Endpoint, strconv.FormatInt(int64(client.client.GetPort()), 10)))
	require.True(t, strings.Contains(cfg.Endpoint, client.client.GetTransport()))
	switch cfg.Version {
	case "v1":
		require.Equal(t, gosnmp.Version1, client.client.GetVersion())
		require.Equal(t, cfg.Community, client.client.GetCommunity())
	case "v2c":
		require.Equal(t, gosnmp.Version2c, client.client.GetVersion())
		require.Equal(t, cfg.Community, client.client.GetCommunity())
	case "v3":
		require.Equal(t, gosnmp.Version3, client.client.GetVersion())
		securityParams := client.client.GetSecurityParameters().(*gosnmp.UsmSecurityParameters)
		require.Equal(t, cfg.User, securityParams.UserName)
		switch cfg.SecurityLevel {
		case "no_auth_no_priv":
			require.Equal(t, gosnmp.NoAuthNoPriv, client.client.GetMsgFlags())
		case "auth_no_priv":
			require.Equal(t, gosnmp.AuthNoPriv, client.client.GetMsgFlags())
			require.Equal(t, cfg.AuthType, securityParams.AuthenticationProtocol)
			require.Equal(t, cfg.AuthPassword, securityParams.AuthenticationPassphrase)
		case "auth_priv":
			require.Equal(t, gosnmp.AuthPriv, client.client.GetMsgFlags())
			require.Equal(t, cfg.AuthType, securityParams.AuthenticationProtocol.String())
			require.Equal(t, cfg.AuthPassword, securityParams.AuthenticationPassphrase)
			require.Equal(t, cfg.PrivacyType, securityParams.PrivacyProtocol.String())
			require.Equal(t, cfg.PrivacyPassword, securityParams.PrivacyPassphrase)
		}
	}
}

func TestConnect(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Good Connect",
			testFunc: func(t *testing.T) {
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Connect", mock.Anything).Return(nil)
				client := &snmpClient{
					logger: &zap.Logger{},
					client: mockGoSNMP,
				}

				err := client.Connect()
				require.NoError(t, err)
			},
		},
		{
			desc: "Bad Connect",
			testFunc: func(t *testing.T) {
				connectErr := errors.New("problem connecting")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Connect", mock.Anything).Return(connectErr)
				client := &snmpClient{
					logger: &zap.Logger{},
					client: mockGoSNMP,
				}

				err := client.Connect()
				require.ErrorIs(t, err, connectErr)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestClose(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "Good Close",
			testFunc: func(t *testing.T) {
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Close", mock.Anything).Return(nil)
				client := &snmpClient{
					logger: &zap.Logger{},
					client: mockGoSNMP,
				}

				err := client.Close()
				require.NoError(t, err)
			},
		},
		{
			desc: "Bad Close",
			testFunc: func(t *testing.T) {
				closeErr := errors.New("problem closing")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Close", mock.Anything).Return(closeErr)
				client := &snmpClient{
					logger: &zap.Logger{},
					client: mockGoSNMP,
				}

				err := client.Close()
				require.ErrorIs(t, err, closeErr)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetScalarData(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "No OIDs does nothing",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetScalarData([]string{}, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client failures adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				getError := errors.New("Bad GET")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Get", []string{"1"}).Return(nil, getError)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: problem with SNMP GET for OIDs '%v': %w", oidSlice, getError)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client timeout failures tries to reset connection",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				getError := errors.New("request timeout (after 0 retries)")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Get", []string{"1"}).Return(nil, getError)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				mockGoSNMP.On("Close", mock.Anything).Return(nil)
				mockGoSNMP.On("Connect", mock.Anything).Return(nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: problem with SNMP GET for OIDs '%v': %w", oidSlice, getError)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client reset connection fails on connect adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				getError := errors.New("request timeout (after 0 retries)")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Get", []string{"1"}).Return(nil, getError)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				mockGoSNMP.On("Close", mock.Anything).Return(nil)
				connectErr := errors.New("can't connect")
				mockGoSNMP.On("Connect", mock.Anything).Return(connectErr)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr1 := fmt.Errorf("problem with getting scalar data: problem with SNMP GET for OIDs '%v': %w", oidSlice, getError)
				expectedErr2 := fmt.Errorf("problem with getting scalar data: problem connecting while trying to reset connection: %w", connectErr)
				expectedErr := fmt.Errorf(expectedErr1.Error() + "; " + expectedErr2.Error())
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client partial failures still return successes",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						oid:       "2",
						value:     int64(1),
						valueType: integerVal,
					},
				}
				pdu1 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "2",
					Type:  gosnmp.Integer,
				}
				getError := errors.New("Bad GET")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("Get", []string{"1"}).
					Return(nil, getError).Once()
				mockGoSNMP.On("Get", []string{"2"}).
					Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1}}, nil).Once()
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(1)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1", "2"}
				badOIDSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: problem with SNMP GET for OIDs '%v': %w", badOIDSlice, getError)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client returned nil value does not return data",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu := gosnmp.SnmpPDU{
					Value: nil,
					Name:  "1",
					Type:  gosnmp.Integer,
				}
				mockGoSNMP.On("Get", []string{"1"}).
					Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				badOID := "1"
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: data for OID '%s' not found", badOID)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client returned unsupported type value does not return data",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu := gosnmp.SnmpPDU{
					Value: true,
					Name:  "1",
					Type:  gosnmp.Boolean,
				}
				mockGoSNMP.On("Get", []string{"1"}).
					Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				badOID := "1"
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: data for OID '%s' not a supported type", badOID)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "Large amount of OIDs handled in chunks",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						oid:       "1",
						value:     int64(1),
						valueType: integerVal,
					},
					{
						oid:       "2",
						value:     int64(1),
						valueType: integerVal,
					},
					{
						oid:       "3",
						value:     int64(1),
						valueType: integerVal,
					},
					{
						oid:       "4",
						value:     int64(1),
						valueType: integerVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu1 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "1",
					Type:  gosnmp.Integer,
				}
				pdu2 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "2",
					Type:  gosnmp.Integer,
				}
				pdu3 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "3",
					Type:  gosnmp.Integer,
				}
				pdu4 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "4",
					Type:  gosnmp.Integer,
				}
				mockGoSNMP.On("Get", []string{"1", "2"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1, pdu2}}, nil)
				mockGoSNMP.On("Get", []string{"3", "4"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu3, pdu4}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1", "2", "3", "4"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client float data type properly converted",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						oid:       "1",
						value:     1.0,
						valueType: floatVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu1 := gosnmp.SnmpPDU{
					Value: 1.0,
					Name:  "1",
					Type:  gosnmp.OpaqueDouble,
				}
				mockGoSNMP.On("Get", []string{"1"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client float data type with bad value adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu1 := gosnmp.SnmpPDU{
					Value: true,
					Name:  "1",
					Type:  gosnmp.OpaqueDouble,
				}
				mockGoSNMP.On("Get", []string{"1"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: data for OID '1' not a supported type")
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client float data type with bad string value adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu1 := gosnmp.SnmpPDU{
					Value: "bad",
					Name:  "1",
					Type:  gosnmp.OpaqueDouble,
				}
				mockGoSNMP.On("Get", []string{"1"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: data for OID '1' not a supported type")
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client int data type with bad value adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu1 := gosnmp.SnmpPDU{
					Value: uint64(math.MaxUint64),
					Name:  "1",
					Type:  gosnmp.Counter64,
				}
				mockGoSNMP.On("Get", []string{"1"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting scalar data: data for OID '1' not a supported type")
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client string data type properly converted",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						oid:       "1",
						value:     "test",
						valueType: stringVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				pdu1 := gosnmp.SnmpPDU{
					Value: []byte("test"),
					Name:  "1",
					Type:  gosnmp.OctetString,
				}
				mockGoSNMP.On("Get", []string{"1"}).Return(&gosnmp.SnmpPacket{Variables: []gosnmp.SnmpPDU{pdu1}}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetScalarData(oidSlice, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}

func TestGetIndexedData(t *testing.T) {
	testCases := []struct {
		desc     string
		testFunc func(*testing.T)
	}{
		{
			desc: "No OIDs does nothing",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{}, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client failures adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				walkError := errors.New("Bad WALK")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				mockGoSNMP.On("BulkWalkAll", "1").Return(nil, walkError)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetIndexedData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: problem with SNMP WALK for OID '1': %w", walkError)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client timeout failures tries to reset connection",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				walkError := errors.New("request timeout (after 0 retries)")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				mockGoSNMP.On("BulkWalkAll", "1").Return(nil, walkError)
				mockGoSNMP.On("Close", mock.Anything).Return(nil)
				mockGoSNMP.On("Connect", mock.Anything).Return(nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetIndexedData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: problem with SNMP WALK for OID '1': %w", walkError)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client reset connection fails on connect adds errors",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				walkError := errors.New("request timeout (after 0 retries)")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				mockGoSNMP.On("BulkWalkAll", "1").Return(nil, walkError)
				mockGoSNMP.On("Close", mock.Anything).Return(nil)
				connectErr := errors.New("can't connect")
				mockGoSNMP.On("Connect", mock.Anything).Return(connectErr)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetIndexedData(oidSlice, &scraperErrors)
				expectedErr1 := fmt.Errorf("problem with getting indexed data: problem with SNMP WALK for OID '1': %w", walkError)
				expectedErr2 := fmt.Errorf("problem with getting indexed data: problem connecting while trying to reset connection: %w", connectErr)
				expectedErr := fmt.Errorf(expectedErr1.Error() + "; " + expectedErr2.Error())
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client partial failures still returns successes",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						columnOID: "2",
						oid:       "2.1",
						value:     int64(1),
						valueType: integerVal,
					},
				}
				pdu1 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "2.1",
					Type:  gosnmp.Integer,
				}
				walkError := errors.New("Bad Walk")
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				mockGoSNMP.On("BulkWalkAll", "1").Return(nil, walkError).Once()
				mockGoSNMP.On("BulkWalkAll", "2").Return([]gosnmp.SnmpPDU{pdu1}, nil).Once()
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1", "2"}
				returnedSNMPData := client.GetIndexedData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: problem with SNMP WALK for OID '1': %w", walkError)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client returned nil value does not return data",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				badOID := "1.1"
				pdu := gosnmp.SnmpPDU{
					Value: nil,
					Name:  badOID,
					Type:  gosnmp.Integer,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetIndexedData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: data for OID '%s' not found", badOID)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client returned unsupported type value does not return data",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				badOID := "1.1"
				pdu := gosnmp.SnmpPDU{
					Value: true,
					Name:  badOID,
					Type:  gosnmp.Boolean,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				oidSlice := []string{"1"}
				returnedSNMPData := client.GetIndexedData(oidSlice, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: data for OID '%s' not a supported type", badOID)
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "Return multiple good values",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						columnOID: "1",
						oid:       "1.1",
						value:     int64(1),
						valueType: integerVal,
					},
					{
						columnOID: "1",
						oid:       "1.2",
						value:     int64(2),
						valueType: integerVal,
					},
					{
						columnOID: "2",
						oid:       "2.1",
						value:     int64(3),
						valueType: integerVal,
					},
					{
						columnOID: "2",
						oid:       "2.2",
						value:     int64(4),
						valueType: integerVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				pdu1 := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "1.1",
					Type:  gosnmp.Integer,
				}
				pdu2 := gosnmp.SnmpPDU{
					Value: 2,
					Name:  "1.2",
					Type:  gosnmp.Integer,
				}
				pdu3 := gosnmp.SnmpPDU{
					Value: 3,
					Name:  "2.1",
					Type:  gosnmp.Integer,
				}
				pdu4 := gosnmp.SnmpPDU{
					Value: 4,
					Name:  "2.2",
					Type:  gosnmp.Integer,
				}

				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu1, pdu2}, nil)
				mockGoSNMP.On("BulkWalkAll", "2").Return([]gosnmp.SnmpPDU{pdu3, pdu4}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1", "2"}, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client float data type properly converted",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						columnOID: "1",
						oid:       "1.1",
						value:     1.0,
						valueType: floatVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				pdu := gosnmp.SnmpPDU{
					Value: 1.0,
					Name:  "1.1",
					Type:  gosnmp.OpaqueDouble,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1"}, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client float data type with bad value adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				pdu := gosnmp.SnmpPDU{
					Value: true,
					Name:  "1.1",
					Type:  gosnmp.OpaqueDouble,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1"}, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: data for OID '1.1' not a supported type")
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client float data type with bad string value adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				pdu := gosnmp.SnmpPDU{
					Value: "bad",
					Name:  "1.1",
					Type:  gosnmp.OpaqueDouble,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1"}, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: data for OID '1.1' not a supported type")
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client int data type with bad value adds error",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				pdu := gosnmp.SnmpPDU{
					Value: uint64(math.MaxUint64),
					Name:  "1.1",
					Type:  gosnmp.Counter64,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1"}, &scraperErrors)
				expectedErr := fmt.Errorf("problem with getting indexed data: data for OID '1.1' not a supported type")
				require.EqualError(t, scraperErrors.Combine(), expectedErr.Error())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client string data type properly converted",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						columnOID: "1",
						oid:       "1.1",
						value:     "test",
						valueType: stringVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version2c)
				pdu := gosnmp.SnmpPDU{
					Value: []byte("test"),
					Name:  "1.1",
					Type:  gosnmp.OctetString,
				}
				mockGoSNMP.On("BulkWalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1"}, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
		{
			desc: "GoSNMP Client v1 uses normal Walk function",
			testFunc: func(t *testing.T) {
				expectedSNMPData := []SNMPData{
					{
						columnOID: "1",
						oid:       "1.1",
						value:     int64(1),
						valueType: integerVal,
					},
				}
				mockGoSNMP := new(mocks.MockGoSNMPWrapper)
				mockGoSNMP.On("GetVersion", mock.Anything).Return(gosnmp.Version1)
				pdu := gosnmp.SnmpPDU{
					Value: 1,
					Name:  "1.1",
					Type:  gosnmp.Counter32,
				}
				mockGoSNMP.On("WalkAll", "1").Return([]gosnmp.SnmpPDU{pdu}, nil)
				mockGoSNMP.On("GetMaxOids", mock.Anything).Return(2)
				client := &snmpClient{
					logger: zap.NewNop(),
					client: mockGoSNMP,
				}
				var scraperErrors scrapererror.ScrapeErrors
				returnedSNMPData := client.GetIndexedData([]string{"1"}, &scraperErrors)
				require.NoError(t, scraperErrors.Combine())
				require.Equal(t, expectedSNMPData, returnedSNMPData)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, tc.testFunc)
	}
}
