// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/api"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/credentials"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/internal/metadata"
)

func setupTestMain(m *testing.M) {
	// Enable the feature gates before all tests to avoid flaky tests.
	err := featuregate.GlobalRegistry().Set(updateCollectorMetadataID, true)
	if err != nil {
		panic("unable to set feature gates")
	}

	code := m.Run()
	os.Exit(code)
}

func TestBasicExtensionConstruction(t *testing.T) {
	t.Parallel()

	testcases := []struct {
		Name    string
		Config  *Config
		WantErr bool
	}{
		{
			Name:    "no_collector_name_causes_error",
			Config:  createDefaultConfig().(*Config),
			WantErr: true,
		},
		{
			Name: "no_credentials_causes_error",
			Config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.CollectorName = "collector_name"
				return cfg
			}(),
			WantErr: true,
		},
		{
			Name: "basic",
			Config: func() *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.CollectorName = "collector_name"
				cfg.Credentials.InstallationToken = "install_token_123456"
				return cfg
			}(),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.Name, func(t *testing.T) {
			se, err := newSumologicExtension(tc.Config, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
			if tc.WantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, se)
			}
		})
	}
}

func TestBasicStart(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(func() http.HandlerFunc {
		var reqCount int32

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO Add payload verification - verify if collectorName is set properly
			reqNum := atomic.AddInt32(&reqCount, 1)

			switch reqNum {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)
				_, err := w.Write([]byte(`{
					"collectorCredentialID": "collectorId",
					"collectorCredentialKey": "collectorKey",
					"collectorId": "id"
				}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case 3:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// should not produce any more requests
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}())
	t.Cleanup(func() { srv.Close() })

	dir := t.TempDir()

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = "collector_name"
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	assert.NotEmpty(t, se.registrationInfo.CollectorCredentialID)
	assert.NotEmpty(t, se.registrationInfo.CollectorCredentialKey)
	assert.NotEmpty(t, se.registrationInfo.CollectorID)
	require.NoError(t, se.Shutdown(context.Background()))
}

func TestStoreCredentials(t *testing.T) {
	t.Parallel()

	getServer := func() *httptest.Server {
		var reqCount int32

		return httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
				// TODO Add payload verification - verify if collectorName is set properly
				reqNum := atomic.AddInt32(&reqCount, 1)

				switch reqNum {
				// register
				case 1:
					assert.Equal(t, registerURL, req.URL.Path)
					_, err := w.Write([]byte(`{
						"collectorCredentialID": "collectorId",
						"collectorCredentialKey": "collectorKey",
						"collectorId": "id"
					}`))
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
					}

				// metadata
				case 2:
					assert.Equal(t, metadataURL, req.URL.Path)
					w.WriteHeader(http.StatusOK)

				// heartbeat
				case 3:
					assert.Equal(t, heartbeatURL, req.URL.Path)
					w.WriteHeader(http.StatusNoContent)

				// should not produce any more requests
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
	}

	getConfig := func(url string) *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.CollectorName = "collector_name"
		cfg.APIBaseURL = url
		cfg.Credentials.InstallationToken = "dummy_install_token"
		return cfg
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	t.Run("dir does not exist", func(t *testing.T) {
		dir := t.TempDir()

		srv := getServer()
		t.Cleanup(func() { srv.Close() })

		cfg := getConfig(srv.URL)
		cfg.CollectorCredentialsDirectory = dir

		// Ensure the directory doesn't exist before running the extension
		require.NoError(t, os.RemoveAll(dir))

		se, err := newSumologicExtension(cfg, logger, component.NewID(metadata.Type), "1.0.0")
		require.NoError(t, err)
		key := createHashKey(cfg)
		fileName, err := credentials.HashKeyToFilename(key)
		require.NoError(t, err)
		credsPath := path.Join(dir, fileName)
		require.NoFileExists(t, credsPath)
		require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
		require.NoError(t, se.Shutdown(context.Background()))
		require.FileExists(t, credsPath)
	})

	t.Run("dir exists before launch with 600 permissions", func(t *testing.T) {
		dir := t.TempDir()

		srv := getServer()
		t.Cleanup(func() { srv.Close() })

		cfg := getConfig(srv.URL)
		cfg.CollectorCredentialsDirectory = dir

		// Ensure the directory has 600 permissions
		require.NoError(t, os.Chmod(dir, 0o600))

		se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
		require.NoError(t, err)
		key := createHashKey(cfg)
		fileName, err := credentials.HashKeyToFilename(key)
		require.NoError(t, err)
		credsPath := path.Join(dir, fileName)
		require.NoFileExists(t, credsPath)
		require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
		require.NoError(t, se.Shutdown(context.Background()))
		require.FileExists(t, credsPath)
	})

	t.Run("ensure dir gets created with 700 permissions", func(t *testing.T) {
		dir := t.TempDir()

		srv := getServer()
		t.Cleanup(func() { srv.Close() })
		cfg := getConfig(srv.URL)
		cfg.CollectorCredentialsDirectory = dir

		// Ensure the directory has 700 permissions
		require.NoError(t, os.Chmod(dir, 0o700))

		se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
		require.NoError(t, err)
		key := createHashKey(cfg)
		fileName, err := credentials.HashKeyToFilename(key)
		require.NoError(t, err)
		credsPath := path.Join(dir, fileName)
		require.NoFileExists(t, credsPath)
		require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
		require.NoError(t, se.Shutdown(context.Background()))
		require.FileExists(t, credsPath)
	})

	t.Run("by default use sha256 for hashing", func(t *testing.T) {
		dir := t.TempDir()

		srv := getServer()
		t.Cleanup(func() { srv.Close() })

		cfg := getConfig(srv.URL)
		cfg.CollectorCredentialsDirectory = dir

		se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
		require.NoError(t, err)
		key := createHashKey(cfg)
		fileName, err := credentials.HashKeyToFilename(key)
		require.NoError(t, err)
		fileNameSha256, err := credentials.HashKeyToFilenameWith(sha256.New(), key)
		require.NoError(t, err)
		require.Equal(t, fileName, fileNameSha256)

		credsPath := path.Join(dir, fileName)
		require.NoFileExists(t, credsPath)
		require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
		require.NoError(t, se.Shutdown(context.Background()))
		require.FileExists(t, credsPath)
	})
}

func TestStoreCredentials_PreexistingCredentialsAreUsed(t *testing.T) {
	t.Parallel()

	var reqCount int32
	getServer := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
				reqNum := atomic.AddInt32(&reqCount, 1)

				switch reqNum {
				// heartbeat
				case 1:
					assert.Equal(t, heartbeatURL, req.URL.Path)
					w.WriteHeader(http.StatusNoContent)

				// metadata
				case 2:
					assert.Equal(t, metadataURL, req.URL.Path)
					w.WriteHeader(http.StatusOK)

				// should not produce any more requests
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
	}

	getConfig := func(url string) *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.CollectorName = "collector_name"
		cfg.APIBaseURL = url
		cfg.Credentials.InstallationToken = "dummy_install_token"
		return cfg
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	dir := t.TempDir()
	t.Logf("Using dir: %s", dir)

	store, err := credentials.NewLocalFsStore(
		credentials.WithCredentialsDirectory(dir),
		credentials.WithLogger(logger),
	)
	require.NoError(t, err)

	srv := getServer()
	t.Cleanup(func() { srv.Close() })

	cfg := getConfig(srv.URL)
	cfg.CollectorCredentialsDirectory = dir

	hashKey := createHashKey(cfg)

	require.NoError(t,
		store.Store(hashKey, credentials.CollectorCredentials{
			CollectorName: "collector_name",
			Credentials: api.OpenRegisterResponsePayload{
				CollectorCredentialID:  "collectorId",
				CollectorCredentialKey: "collectorKey",
				CollectorID:            "id",
			},
		}),
	)

	se, err := newSumologicExtension(cfg, logger, component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)

	fileName, err := credentials.HashKeyToFilename(hashKey)
	require.NoError(t, err)
	credsPath := path.Join(dir, fileName)
	// Credentials file exists before starting the extension because we created
	// it directly via store.Store()
	require.FileExists(t, credsPath)

	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, se.Shutdown(context.Background()))
	require.FileExists(t, credsPath)

	require.EqualValues(t, 2, atomic.LoadInt32(&reqCount))
}

func TestLocalFSCredentialsStore_WorkCorrectlyForMultipleExtensions(t *testing.T) {
	t.Parallel()

	getServer := func() *httptest.Server {
		var reqCount int32

		return httptest.NewServer(http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
				// TODO Add payload verification - verify if collectorName is set properly
				reqNum := atomic.AddInt32(&reqCount, 1)

				switch reqNum {
				// register
				case 1:
					assert.Equal(t, registerURL, req.URL.Path)
					_, err := w.Write([]byte(`{
						"collectorCredentialID": "collectorId",
						"collectorCredentialKey": "collectorKey",
						"collectorId": "id"
					}`))
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
					}

				// metadata
				case 2:
					assert.Equal(t, metadataURL, req.URL.Path)
					w.WriteHeader(http.StatusOK)

				// heartbeat
				case 3:
					assert.Equal(t, heartbeatURL, req.URL.Path)
					w.WriteHeader(http.StatusNoContent)

				// should not produce any more requests
				default:
					w.WriteHeader(http.StatusInternalServerError)
				}
			}))
	}

	getConfig := func(url string) *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.CollectorName = "collector_name"
		cfg.APIBaseURL = url
		cfg.Credentials.InstallationToken = "dummy_install_token"
		return cfg
	}

	getDir := func(t *testing.T) string {
		return t.TempDir()
	}

	dir1 := getDir(t)
	dir2 := getDir(t)

	srv1 := getServer()
	t.Cleanup(func() { srv1.Close() })
	srv2 := getServer()
	t.Cleanup(func() { srv2.Close() })

	cfg1 := getConfig(srv1.URL)
	cfg1.CollectorCredentialsDirectory = dir1

	cfg2 := getConfig(srv2.URL)
	cfg2.CollectorCredentialsDirectory = dir2

	logger1, err := zap.NewDevelopment(zap.Fields(zap.Int("#", 1)))
	require.NoError(t, err)

	logger2, err := zap.NewDevelopment(zap.Fields(zap.Int("#", 2)))
	require.NoError(t, err)

	se1, err := newSumologicExtension(cfg1, logger1, component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, se1.Shutdown(context.Background())) })
	fileName1, err := credentials.HashKeyToFilename(createHashKey(cfg1))
	require.NoError(t, err)
	credsPath1 := path.Join(dir1, fileName1)
	require.NoFileExists(t, credsPath1)
	require.NoError(t, se1.Start(context.Background(), componenttest.NewNopHost()))
	require.FileExists(t, credsPath1)

	se2, err := newSumologicExtension(cfg2, logger2, component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, se2.Shutdown(context.Background())) })
	fileName2, err := credentials.HashKeyToFilename(createHashKey(cfg2))
	require.NoError(t, err)
	credsPath2 := path.Join(dir2, fileName2)
	require.NoFileExists(t, credsPath2)
	require.NoError(t, se2.Start(context.Background(), componenttest.NewNopHost()))
	require.FileExists(t, credsPath2)

	require.NotEqual(t, credsPath1, credsPath2,
		"credentials files should be different for configs with different apiBaseURLs",
	)
}

func TestRegisterEmptyCollectorName(t *testing.T) {
	t.Parallel()

	hostname, err := getHostname(zap.NewNop())
	require.NoError(t, err)
	srv := httptest.NewServer(func() http.HandlerFunc {
		var reqCount int32

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO Add payload verification - verify if collectorName is set properly
			reqNum := atomic.AddInt32(&reqCount, 1)

			switch reqNum {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err = w.Write([]byte(`{
					"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
					"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					"collectorId": "000000000FFFFFFF"
				}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case 3:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// should not produce any more requests
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}())

	dir := t.TempDir()
	t.Cleanup(func() {
		srv.Close()
	})

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err)
	require.Equal(t, hostname, se.collectorName)
}

func TestRegisterEmptyCollectorNameForceRegistration(t *testing.T) {
	t.SkipNow() // Skip this test for now as it is flaky
	t.Parallel()

	hostname, err := getHostname(zap.NewNop())
	require.NoError(t, err)
	srv := httptest.NewServer(func() http.HandlerFunc {
		var reqCount int32

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO Add payload verification - verify if collectorName is set properly
			reqNum := atomic.AddInt32(&reqCount, 1)

			switch reqNum {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err = w.Write([]byte(`{
					"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
					"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					"collectorId": "000000000FFFFFFF",
					"collectorName": "hostname-test-123456123123"
				}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// register again because force registration was set
			case 3:
				assert.Equal(t, registerURL, req.URL.Path)

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err = w.Write([]byte(`{
					"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
					"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					"collectorId": "000000000FFFFFFF",
					"collectorName": "hostname-test-123456123123"
				}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

			// metadata
			case 4:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// should not produce any more requests
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}())

	dir := t.TempDir()
	t.Cleanup(func() {
		srv.Close()
	})

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir
	cfg.ForceRegistration = true

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, se.Shutdown(context.Background()))
	assert.NotEmpty(t, se.collectorName)
	assert.Equal(t, hostname, se.collectorName)
	colCreds, err := se.credentialsStore.Get(se.hashKey)
	require.NoError(t, err)
	colName := colCreds.CollectorName
	se, err = newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	assert.Equal(t, se.collectorName, colName)
}

func TestCollectorSendsBasicAuthHeadersOnRegistration(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(func() http.HandlerFunc {
		var reqCount int32

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO Add payload verification - verify if collectorName is set properly
			reqNum := atomic.AddInt32(&reqCount, 1)

			switch reqNum {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err := w.Write([]byte(`{
					"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
					"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					"collectorId": "000000000FFFFFFF"
				}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case 3:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// should not produce any more requests
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}())

	t.Cleanup(func() { srv.Close() })

	dir := t.TempDir()

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, se.Shutdown(context.Background()))
}

func TestCollectorCheckingCredentialsFoundInLocalStorage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cStore, err := credentials.NewLocalFsStore(
		credentials.WithCredentialsDirectory(dir),
		credentials.WithLogger(zap.NewNop()),
	)
	require.NoError(t, err)

	storeCredentials := func(t *testing.T, url string) {
		creds := credentials.CollectorCredentials{
			CollectorName: "test-name",
			Credentials: api.OpenRegisterResponsePayload{
				CollectorName:          "test-name",
				CollectorID:            "test-id",
				CollectorCredentialID:  "test-credential-id",
				CollectorCredentialKey: "test-credential-key",
			},
			APIBaseURL: url,
		}
		storageKey := createHashKey(&Config{
			CollectorName: "test-name",
			Credentials: accessCredentials{
				InstallationToken: "dummy_install_token",
			},
			APIBaseURL: url,
		})
		t.Logf("Storing collector credentials in temp dir: %s", dir)
		require.NoError(t, cStore.Store(storageKey, creds))
	}

	testcases := []struct {
		name             string
		expectedReqCount int32
		srvFn            func() (*httptest.Server, *int32)
		configFn         func(url string) *Config
	}{
		{
			name:             "collector checks found credentials via heartbeat call - no registration is done",
			expectedReqCount: 3,
			srvFn: func() (*httptest.Server, *int32) {
				var reqCount int32

				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						reqNum := atomic.AddInt32(&reqCount, 1)

						switch reqNum {
						// heartbeat
						case 1:
							assert.NotEqual(t, registerURL, req.URL.Path,
								"collector shouldn't call the register API when credentials locally retrieved")

							assert.Equal(t, heartbeatURL, req.URL.Path)

							authHeader := req.Header.Get("Authorization")
							token := base64.StdEncoding.EncodeToString(
								[]byte("test-credential-id:test-credential-key"),
							)
							assert.Equal(t, "Basic "+token, authHeader,
								"collector didn't send correct Authorization header with heartbeat request")

							w.WriteHeader(http.StatusNoContent)

						// metadata
						case 2:
							assert.Equal(t, metadataURL, req.URL.Path)
							w.WriteHeader(http.StatusOK)

						// should not produce any more requests
						default:
							w.WriteHeader(http.StatusInternalServerError)
						}
					})),
					&reqCount
			},
			configFn: func(url string) *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.CollectorName = "test-name"
				cfg.APIBaseURL = url
				cfg.Credentials.InstallationToken = "dummy_install_token"
				cfg.CollectorCredentialsDirectory = dir
				return cfg
			},
		},
		{
			name:             "collector checks network issues - no registration is done",
			expectedReqCount: 4,
			srvFn: func() (*httptest.Server, *int32) {
				var reqCount int32

				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						reqNum := atomic.AddInt32(&reqCount, 1)

						switch reqNum {
						// failing heartbeat
						case 1:
							assert.NotEqual(t, registerURL, req.URL.Path,
								"collector shouldn't call the register API when credentials locally retrieved")

							assert.Equal(t, heartbeatURL, req.URL.Path)

							authHeader := req.Header.Get("Authorization")
							token := base64.StdEncoding.EncodeToString(
								[]byte("test-credential-id:test-credential-key"),
							)
							assert.Equal(t, "Basic "+token, authHeader,
								"collector didn't send correct Authorization header with heartbeat request")

							w.WriteHeader(http.StatusInternalServerError)

						// successful heartbeat
						case 2:
							assert.NotEqual(t, registerURL, req.URL.Path,
								"collector shouldn't call the register API when credentials locally retrieved")

							assert.Equal(t, heartbeatURL, req.URL.Path)

							authHeader := req.Header.Get("Authorization")
							token := base64.StdEncoding.EncodeToString(
								[]byte("test-credential-id:test-credential-key"),
							)
							assert.Equal(t, "Basic "+token, authHeader,
								"collector didn't send correct Authorization header with heartbeat request")

							w.WriteHeader(http.StatusNoContent)

						// metadata
						case 3:
							assert.Equal(t, metadataURL, req.URL.Path)
							w.WriteHeader(http.StatusOK)

						// should not produce any more requests
						default:
							w.WriteHeader(http.StatusInternalServerError)
						}
					})),
					&reqCount
			},
			configFn: func(url string) *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.CollectorName = "test-name"
				cfg.APIBaseURL = url
				cfg.Credentials.InstallationToken = "dummy_install_token"
				cfg.CollectorCredentialsDirectory = dir
				return cfg
			},
		},
		{
			name:             "collector checks network issues - registration is done",
			expectedReqCount: 4,
			srvFn: func() (*httptest.Server, *int32) {
				var reqCount int32

				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						reqNum := atomic.AddInt32(&reqCount, 1)

						switch reqNum {
						// failing heartbeat
						case 1:
							assert.NotEqual(t, registerURL, req.URL.Path,
								"collector shouldn't call the register API when credentials locally retrieved")

							assert.Equal(t, heartbeatURL, req.URL.Path)

							authHeader := req.Header.Get("Authorization")
							token := base64.StdEncoding.EncodeToString(
								[]byte("test-credential-id:test-credential-key"),
							)
							assert.Equal(t, "Basic "+token, authHeader,
								"collector didn't send correct Authorization header with heartbeat request")

							w.WriteHeader(http.StatusUnauthorized)

						// register
						case 2:
							assert.Equal(t, registerURL, req.URL.Path)

							authHeader := req.Header.Get("Authorization")
							assert.Equal(t, "Bearer dummy_install_token", authHeader,
								"collector didn't send correct Authorization header with registration request")

							_, err := w.Write([]byte(`{
							"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
							"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
							"collectorId": "000000000FFFFFFF"
						}`))
							if err != nil {
								w.WriteHeader(http.StatusInternalServerError)
							}

						// metadata
						case 3:
							w.WriteHeader(http.StatusOK)

						// heartbeat
						case 4:
							w.WriteHeader(http.StatusNoContent)

						// should not produce any more requests
						default:
							w.WriteHeader(http.StatusInternalServerError)
						}
					})),
					&reqCount
			},
			configFn: func(url string) *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.CollectorName = "test-name"
				cfg.APIBaseURL = url
				cfg.Credentials.InstallationToken = "dummy_install_token"
				cfg.CollectorCredentialsDirectory = dir
				return cfg
			},
		},
		{
			name:             "collector registers when no matching credentials are found in local storage",
			expectedReqCount: 3,
			srvFn: func() (*httptest.Server, *int32) {
				var reqCount int32

				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
						reqNum := atomic.AddInt32(&reqCount, 1)

						switch reqNum {
						// register
						case 1:
							assert.Equal(t, registerURL, req.URL.Path)

							authHeader := req.Header.Get("Authorization")
							assert.Equal(t, "Bearer dummy_install_token", authHeader,
								"collector didn't send correct Authorization header with registration request")

							_, err := w.Write([]byte(`{
							"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
							"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
							"collectorId": "000000000FFFFFFF"
						}`))
							if err != nil {
								w.WriteHeader(http.StatusInternalServerError)
							}

						// metadata
						case 2:
							w.WriteHeader(http.StatusOK)

						// heartbeat
						case 3:
							w.WriteHeader(http.StatusNoContent)

						// should not produce any more requests
						default:
							w.WriteHeader(http.StatusInternalServerError)
						}
					})),
					&reqCount
			},
			configFn: func(url string) *Config {
				cfg := createDefaultConfig().(*Config)
				cfg.CollectorName = "test-name-not-in-the-credentials-store"
				cfg.APIBaseURL = url
				cfg.Credentials.InstallationToken = "dummy_install_token"
				cfg.CollectorCredentialsDirectory = dir
				return cfg
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			srv, reqCount := tc.srvFn()
			t.Cleanup(func() { srv.Close() })

			cfg := tc.configFn(srv.URL)
			storeCredentials(t, srv.URL)

			logger, err := zap.NewDevelopment()
			require.NoError(t, err)

			se, err := newSumologicExtension(cfg, logger, component.NewID(metadata.Type), "1.0.0")
			require.NoError(t, err)
			require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))

			if !assert.Eventually(t,
				func() bool {
					return atomic.LoadInt32(reqCount) == tc.expectedReqCount
				},
				5*time.Second, 100*time.Millisecond,
			) {
				t.Logf("the expected number of requests (%d) wasn't reached, only got %d",
					tc.expectedReqCount, atomic.LoadInt32(reqCount),
				)
			}

			require.NoError(t, se.Shutdown(context.Background()))
		})
	}
}

func TestRegisterEmptyCollectorNameWithBackoff(t *testing.T) {
	var retriesLimit int32 = 5
	t.Parallel()

	hostname, err := getHostname(zap.NewNop())
	require.NoError(t, err)
	srv := httptest.NewServer(func() http.HandlerFunc {
		var reqCount int32

		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO Add payload verification - verify if collectorName is set properly
			reqNum := atomic.AddInt32(&reqCount, 1)

			switch {
			// register
			case reqNum <= retriesLimit:
				assert.Equal(t, registerURL, req.URL.Path)

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				if reqCount < retriesLimit {
					w.WriteHeader(http.StatusTooManyRequests)
				} else {
					_, err = w.Write([]byte(`{
						"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
						"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
						"collectorId": "000000000FFFFFFF"
					}`))
					if err != nil {
						w.WriteHeader(http.StatusInternalServerError)
					}
				}

			// metadata
			case reqNum == retriesLimit+1:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case reqNum == retriesLimit+2:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// should not produce any more requests
			default:
				w.WriteHeader(http.StatusInternalServerError)
			}
		})
	}())

	dir := t.TempDir()
	t.Cleanup(func() {
		srv.Close()
	})
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir
	cfg.BackOff.InitialInterval = time.Millisecond
	cfg.BackOff.MaxInterval = time.Millisecond

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	require.Equal(t, hostname, se.collectorName)
}

func TestRegisterEmptyCollectorNameUnrecoverableError(t *testing.T) {
	t.Parallel()

	hostname, err := getHostname(zap.NewNop())
	require.NoError(t, err)
	srv := httptest.NewServer(func() http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// TODO Add payload verification - verify if collectorName is set properly
			assert.Equal(t, registerURL, req.URL.Path)

			authHeader := req.Header.Get("Authorization")
			assert.Equal(t, "Bearer dummy_install_token", authHeader,
				"collector didn't send correct Authorization header with registration request")

			w.WriteHeader(http.StatusNotFound)
			_, err = w.Write([]byte(`{
				"id": "XXXXX-XXXXX-XXXXX",
				"errors": [
					{
						"code": "collector-registration:dummy_error",
						"message": "The collector cannot be registered"
					}
				]
			}`))
			assert.NoError(t, err)
		})
	}())

	dir := t.TempDir()
	t.Cleanup(func() {
		srv.Close()
	})
	require.NoError(t, err)

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir
	cfg.BackOff.InitialInterval = time.Millisecond
	cfg.BackOff.MaxInterval = time.Millisecond

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.EqualError(t, se.Start(context.Background(), componenttest.NewNopHost()),
		"collector registration failed: failed to register the collector, got HTTP status code: 404")
	require.Equal(t, hostname, se.collectorName)
}

func TestRegistrationRedirect(t *testing.T) {
	t.Parallel()

	var destReqCount int32
	destSrv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			switch atomic.AddInt32(&destReqCount, 1) {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)

				authHeader := req.Header.Get("Authorization")

				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err := w.Write([]byte(`{
					"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
					"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					"collectorId": "000000000FFFFFFF"
				}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}

			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case 3:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// heartbeat
			case 4:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// metadata
			case 5:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case 6:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// should not produce any more requests
			default:
				assert.Fail(t,
					"extension should not make more than 5 requests to the destination server",
				)
			}
		},
	))
	t.Cleanup(func() { destSrv.Close() })

	var origReqCount int32
	origSrv := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, req *http.Request) {
			switch atomic.AddInt32(&origReqCount, 1) {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)
				http.Redirect(w, req, destSrv.URL, http.StatusMovedPermanently)

			// should not produce any more requests
			default:
				assert.Fail(t,
					"extension should not make more than 1 request to the original server",
				)
			}
		},
	))
	t.Cleanup(func() { origSrv.Close() })

	dir := t.TempDir()

	configFn := func() *Config {
		cfg := createDefaultConfig().(*Config)
		cfg.CollectorName = ""
		cfg.APIBaseURL = origSrv.URL
		cfg.Credentials.InstallationToken = "dummy_install_token"
		cfg.CollectorCredentialsDirectory = dir
		return cfg
	}

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	t.Run("works correctly", func(t *testing.T) {
		se, err := newSumologicExtension(configFn(), logger, component.NewID(metadata.Type), "1.0.0")
		require.NoError(t, err)
		require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
		assert.Eventually(t, func() bool { return atomic.LoadInt32(&origReqCount) == 1 },
			5*time.Second, 100*time.Millisecond,
			"extension should only make 1 request to the original server before redirect",
		)
		assert.Eventually(t, func() bool { return atomic.LoadInt32(&destReqCount) == 3 },
			5*time.Second, 100*time.Millisecond,
			"extension should make 3 requests (registration + metadata + heartbeat) to the destination server",
		)
		require.NoError(t, se.Shutdown(context.Background()))
	})

	t.Run("credentials store retrieves credentials with redirected api url", func(t *testing.T) {
		se, err := newSumologicExtension(configFn(), logger, component.NewID(metadata.Type), "1.0.0")
		require.NoError(t, err)
		require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))

		assert.Eventually(t, func() bool { return atomic.LoadInt32(&origReqCount) == 1 },
			5*time.Second, 100*time.Millisecond,
			"after restarting with locally stored credentials extension shouldn't call the original server",
		)

		assert.Eventually(t, func() bool { return atomic.LoadInt32(&destReqCount) == 6 },
			5*time.Second, 100*time.Millisecond,
			"extension should make 6 requests (registration + metadata + heartbeat, after restart "+
				"heartbeat to validate credentials, metadata update, and then the first heartbeat on "+
				"which we wait here) to the destination server",
		)

		require.NoError(t, se.Shutdown(context.Background()))
	})
}

func TestCollectorReregistersAfterHTTPUnauthorizedFromHeartbeat(t *testing.T) {
	t.Parallel()

	var reqCount int32
	srv := httptest.NewServer(func() http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			reqNum := atomic.AddInt32(&reqCount, 1)

			t.Logf("request: (#%d) %s", reqNum, req.URL.Path)
			handlerRegister := func() {
				require.Equal(t, registerURL, req.URL.Path, "request num 1")

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err := w.Write([]byte(`{
					"collectorCredentialID": "aaaaaaaaaaaaaaaaaaaa",
					"collectorCredentialKey": "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
					"collectorId": "000000000FFFFFFF",
					"collectorName": "hostname-test-123456123123"
					}`))
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
				}
			}

			switch reqNum {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)
				handlerRegister()

			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)

			// heartbeat
			case 3:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)

			// heartbeat
			case 4:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				// return unauthorized to mimic collector being removed from API
				w.WriteHeader(http.StatusUnauthorized)

			// register
			case 5:
				assert.Equal(t, registerURL, req.URL.Path)
				handlerRegister()

			default:
				assert.Equal(t, heartbeatURL, req.URL.Path)
				w.WriteHeader(http.StatusNoContent)
			}
		})
	}())

	t.Cleanup(func() { srv.Close() })

	dir := t.TempDir()

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir
	cfg.HeartBeatInterval = 100 * time.Millisecond

	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	se, err := newSumologicExtension(cfg, logger, component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))

	const expectedReqCount = 10
	if !assert.Eventually(t,
		func() bool {
			return atomic.LoadInt32(&reqCount) == expectedReqCount
		},
		5*time.Second, 50*time.Millisecond,
	) {
		t.Logf("the expected number of requests (%d) wasn't reached, got %d",
			expectedReqCount, atomic.LoadInt32(&reqCount),
		)
	}

	require.NoError(t, se.Shutdown(context.Background()))
}

func TestRegistrationRequestPayload(t *testing.T) {
	t.Parallel()

	hostname, err := getHostname(zap.NewNop())
	require.NoError(t, err)
	var reqCount int32
	srv := httptest.NewServer(func() http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			reqNum := atomic.AddInt32(&reqCount, 1)

			switch reqNum {
			// register
			case 1:
				assert.Equal(t, registerURL, req.URL.Path)

				var reqPayload api.OpenRegisterRequestPayload
				assert.NoError(t, json.NewDecoder(req.Body).Decode(&reqPayload))
				assert.True(t, reqPayload.Clobber)
				assert.Equal(t, hostname, reqPayload.Hostname)
				assert.Equal(t, "my description", reqPayload.Description)
				assert.Equal(t, "my category/", reqPayload.Category)
				assert.Equal(t,
					map[string]any{
						"field1": "value1",
						"field2": "value2",
					},
					reqPayload.Fields,
				)
				assert.Equal(t, "PST", reqPayload.TimeZone)

				authHeader := req.Header.Get("Authorization")
				assert.Equal(t, "Bearer dummy_install_token", authHeader,
					"collector didn't send correct Authorization header with registration request")

				_, err = w.Write([]byte(`{
					"collectorCredentialID": "mycredentialID",
					"collectorCredentialKey": "mycredentialKey",
					"collectorId": "0000000001231231",
					"collectorName": "otc-test-123456123123"
					}`))
				assert.NoError(t, err)
			// metadata
			case 2:
				assert.Equal(t, metadataURL, req.URL.Path)
				w.WriteHeader(http.StatusOK)
			}
		})
	}())

	dir := t.TempDir()
	t.Cleanup(func() {
		srv.Close()
	})

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.CollectorCredentialsDirectory = dir
	cfg.BackOff.InitialInterval = time.Millisecond
	cfg.BackOff.MaxInterval = time.Millisecond
	cfg.Clobber = true
	cfg.CollectorDescription = "my description"
	cfg.CollectorCategory = "my category/"
	cfg.CollectorFields = map[string]any{
		"field1": "value1",
		"field2": "value2",
	}
	cfg.TimeZone = "PST"

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)
	require.NoError(t, se.Start(context.Background(), componenttest.NewNopHost()))
	require.Equal(t, hostname, se.collectorName)

	require.NoError(t, se.Shutdown(context.Background()))
}

func TestWatchCredentialKey(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Credentials.InstallationToken = "dummy_install_token"
	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)

	ctx := context.Background()
	ctxc, cancel := context.WithCancel(ctx)
	cancel()
	v := se.WatchCredentialKey(ctxc, "")
	require.Empty(t, v)

	v = se.WatchCredentialKey(context.Background(), "foobar")
	require.Empty(t, v)

	go func() {
		time.Sleep(time.Millisecond * 100)
		se.credsNotifyLock.Lock()
		defer se.credsNotifyLock.Unlock()
		se.registrationInfo.CollectorCredentialKey = "test-credential-key"
		close(se.credsNotifyUpdate)
	}()

	v = se.WatchCredentialKey(context.Background(), "")
	require.Equal(t, "test-credential-key", v)
}

func TestCreateCredentialsHeader(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Credentials.InstallationToken = "dummy_install_token"
	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)

	_, err = se.CreateCredentialsHeader()
	require.Error(t, err)

	se.registrationInfo.CollectorCredentialID = "test-credential-id"
	se.registrationInfo.CollectorCredentialKey = "test-credential-key"

	h, err := se.CreateCredentialsHeader()
	require.NoError(t, err)

	authHeader := h.Get("Authorization")
	token := base64.StdEncoding.EncodeToString(
		[]byte("test-credential-id:test-credential-key"),
	)
	assert.Equal(t, "Basic "+token, authHeader)
}

func TestGetHostIpAddress(t *testing.T) {
	ip, err := baseURL()
	require.NoError(t, err)
	require.NotEmpty(t, ip)
}

func TestUpdateMetadataRequestPayload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(func() http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			assert.Equal(t, metadataURL, req.URL.Path)

			var reqPayload api.OpenMetadataRequestPayload
			assert.NoError(t, json.NewDecoder(req.Body).Decode(&reqPayload))
			assert.NotEmpty(t, reqPayload.HostDetails.Name)
			assert.NotEmpty(t, reqPayload.HostDetails.OsName)
			// @sumo-drosiek: It happened to be empty OsVersion on my machine
			// require.NotEmpty(t, reqPayload.HostDetails.OsVersion)
			assert.NotEmpty(t, reqPayload.NetworkDetails.HostIPAddress)
			assert.Equal(t, "EKS-1.20.2", reqPayload.HostDetails.Environment)
			assert.Equal(t, "1.0.0", reqPayload.CollectorDetails.RunningVersion)
			assert.EqualValues(t, "A", reqPayload.TagDetails["team"])
			assert.EqualValues(t, "linux", reqPayload.TagDetails["app"])
			assert.EqualValues(t, "true", reqPayload.TagDetails["sumo.disco.enabled"])

			_, err := w.Write([]byte(``))

			assert.NoError(t, err)
		})
	}())

	cfg := createDefaultConfig().(*Config)
	cfg.CollectorName = ""
	cfg.APIBaseURL = srv.URL
	cfg.Credentials.InstallationToken = "dummy_install_token"
	cfg.BackOff.InitialInterval = time.Millisecond
	cfg.BackOff.MaxInterval = time.Millisecond
	cfg.Clobber = true
	cfg.CollectorEnvironment = "EKS-1.20.2"
	cfg.CollectorDescription = "my description"
	cfg.CollectorCategory = "my category/"
	cfg.CollectorFields = map[string]any{
		"team": "A",
		"app":  "linux",
	}
	cfg.DiscoverCollectorTags = true
	cfg.TimeZone = "PST"

	se, err := newSumologicExtension(cfg, zap.NewNop(), component.NewID(metadata.Type), "1.0.0")
	require.NoError(t, err)

	httpClient, err := se.getHTTPClient(context.Background(), se.conf.ClientConfig, api.OpenRegisterResponsePayload{})
	require.NoError(t, err)

	err = se.updateMetadataWithHTTPClient(context.TODO(), httpClient)
	require.NoError(t, err)
}

func Test_cleanupBuildVersion(t *testing.T) {
	type args struct {
		version string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "already ok",
			args: args{version: "0.108.0-sumo-2"},
			want: "v0.108.0-sumo-2",
		}, {
			name: "no hash fips",
			args: args{version: "0.108.0-sumo-2-fips"},
			want: "v0.108.0-sumo-2-fips",
		}, {
			name: "with hash",
			args: args{version: "0.108.0-sumo-2-4d57200692d5c5c39effad4ae3b29fef79209113"},
			want: "v0.108.0-sumo-2",
		}, {
			name: "hash fips",
			args: args{version: "0.108.0-sumo-2-4d57200692d5c5c39effad4ae3b29fef79209113-fips"},
			want: "v0.108.0-sumo-2-fips",
		}, {
			name: "v already ok",
			args: args{version: "v0.108.0-sumo-2"},
			want: "v0.108.0-sumo-2",
		}, {
			name: "v no hash fips",
			args: args{version: "v0.108.0-sumo-2-fips"},
			want: "v0.108.0-sumo-2-fips",
		}, {
			name: "v with hash",
			args: args{version: "v0.108.0-sumo-2-4d57200692d5c5c39effad4ae3b29fef79209113"},
			want: "v0.108.0-sumo-2",
		}, {
			name: "v hash fips",
			args: args{version: "v0.108.0-sumo-2-4d57200692d5c5c39effad4ae3b29fef79209113-fips"},
			want: "v0.108.0-sumo-2-fips",
		}, {
			name: "no patch version",
			args: args{version: "0.108-sumo-2"},
			want: "0.108-sumo-2",
		}, {
			name: "v no patch version",
			args: args{version: "v0.108-sumo-2"},
			want: "v0.108-sumo-2",
		}, {
			name: "no sumo version",
			args: args{version: "0.108-0-sumo"},
			want: "0.108-0-sumo",
		}, {
			name: "v no patch version",
			args: args{version: "v0.108-0-sumo"},
			want: "v0.108-0-sumo",
		}, {
			name: "nonsense",
			args: args{version: "hfiwe-23rhc8eg.fhf"},
			want: "hfiwe-23rhc8eg.fhf",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, cleanupBuildVersion(tt.args.version), "cleanupBuildVersion(%v)", tt.args.version)
		})
	}
}
