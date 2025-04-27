// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension"

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Showmax/go-fqdn"
	"github.com/cenkalti/backoff/v4"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/process"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensionauth"
	"go.opentelemetry.io/collector/featuregate"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/api"
	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension/credentials"
)

type SumologicExtension struct {
	collectorName string
	buildVersion  string

	// The lock around baseURL is needed because sumologicexporter is using
	// it as base URL for API requests and this access has to be coordinated.
	baseURLLock sync.RWMutex
	baseURL     string

	credsNotifyLock   sync.Mutex
	credsNotifyUpdate chan struct{}

	host             component.Host
	conf             *Config
	origLogger       *zap.Logger
	logger           *zap.Logger
	credentialsStore credentials.Store
	hashKey          string
	httpClient       *http.Client
	registrationInfo api.OpenRegisterResponsePayload
	updateMetadata   bool

	stickySessionCookieLock sync.RWMutex
	stickySessionCookie     string

	closeChan chan struct{}
	closeOnce sync.Once
	backOff   *backoff.ExponentialBackOff
	id        component.ID
}

const (
	heartbeatURL = "/api/v1/collector/heartbeat"
	metadataURL  = "/api/v1/otCollectors/metadata"
	registerURL  = "/api/v1/collector/register"

	collectorIDField           = "collector_id"
	collectorNameField         = "collector_name"
	collectorCredentialIDField = "collector_credential_id"

	stickySessionKey = "AWSALB"

	activeMQJavaProcess      = "activemq.jar"
	cassandraJavaProcess     = "org.apache.cassandra.service.CassandraDaemon"
	dockerDesktopJavaProcess = "com.docker.backend"
	jmxJavaProcess           = "com.sun.management.jmxremote"
)

const (
	updateCollectorMetadataID    = "extension.sumologic.updateCollectorMetadata"
	updateCollectorMetadataStage = featuregate.StageAlpha

	DefaultHeartbeatInterval = 15 * time.Second
)

var updateCollectorMetadataFeatureGate *featuregate.Gate

func init() {
	updateCollectorMetadataFeatureGate = featuregate.GlobalRegistry().MustRegister(
		updateCollectorMetadataID,
		updateCollectorMetadataStage,
		featuregate.WithRegisterDescription("When enabled, the collector will update its Sumo Logic metadata on startup."),
		featuregate.WithRegisterReferenceURL("https://github.com/SumoLogic/sumologic-otel-collector/pull/858"),
	)
}

// SumologicExtension implements extensionauth.HTTPClient
var (
	_ extension.Extension      = (*SumologicExtension)(nil)
	_ extensionauth.HTTPClient = (*SumologicExtension)(nil)
)

func newSumologicExtension(conf *Config, logger *zap.Logger, id component.ID, buildVersion string) (*SumologicExtension, error) {
	if conf.Credentials.InstallationToken == "" {
		return nil, errors.New("access credentials not provided: need installation_token")
	}

	hostname, err := getHostname(logger)
	if err != nil {
		return nil, err
	}

	credentialsStore, err := credentials.NewLocalFsStore(
		credentials.WithCredentialsDirectory(conf.CollectorCredentialsDirectory),
		credentials.WithLogger(logger),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize credentials store: %w", err)
	}

	var (
		collectorName string
		hashKey       = createHashKey(conf)
	)
	if conf.CollectorName == "" {
		// If collector name is not set by the user, check if the collector was restarted
		// and that we can reuse collector name save in credentials store.
		if creds, err := credentialsStore.Get(hashKey); err != nil {
			// If credentials file is not stored on filesystem generate collector name
			collectorName = hostname
		} else {
			collectorName = creds.CollectorName
		}
	} else {
		collectorName = conf.CollectorName
	}

	if conf.HeartBeatInterval <= 0 {
		conf.HeartBeatInterval = DefaultHeartbeatInterval
	}

	// Prepare ExponentialBackoff
	backOff := backoff.NewExponentialBackOff()
	backOff.InitialInterval = conf.BackOff.InitialInterval
	backOff.MaxElapsedTime = conf.BackOff.MaxElapsedTime
	backOff.MaxInterval = conf.BackOff.MaxInterval

	return &SumologicExtension{
		collectorName:     collectorName,
		buildVersion:      buildVersion,
		baseURL:           strings.TrimSuffix(conf.APIBaseURL, "/"),
		credsNotifyUpdate: make(chan struct{}),
		conf:              conf,
		origLogger:        logger,
		logger:            logger,
		hashKey:           hashKey,
		credentialsStore:  credentialsStore,
		updateMetadata:    updateCollectorMetadataFeatureGate.IsEnabled(),
		closeChan:         make(chan struct{}),
		backOff:           backOff,
		id:                id,
	}, nil
}

func createHashKey(conf *Config) string {
	return fmt.Sprintf("%s%s%s",
		conf.CollectorName,
		conf.Credentials.InstallationToken,
		strings.TrimSuffix(conf.APIBaseURL, "/"),
	)
}

func (se *SumologicExtension) Start(ctx context.Context, host component.Host) error {
	var err error
	se.host = host

	// if force registration is not enabled, verify that the store is correctly configured
	if !se.conf.ForceRegistration {
		err = se.credentialsStore.Validate()
		if err != nil {
			return err
		}
	}

	colCreds, err := se.getCredentials(ctx)
	if err != nil {
		return err
	}

	if err = se.injectCredentials(ctx, colCreds); err != nil {
		return err
	}

	// Add logger fields based on actual collector name and ID.
	se.logger = se.origLogger.With(
		zap.String(collectorNameField, colCreds.Credentials.CollectorName),
		zap.String(collectorIDField, colCreds.Credentials.CollectorID),
	)

	if se.updateMetadata {
		err = se.updateMetadataWithBackoff(ctx)
		if err != nil {
			return err
		}
	}

	go se.heartbeatLoop()

	return nil
}

// Shutdown is invoked during service shutdown.
func (se *SumologicExtension) Shutdown(ctx context.Context) error {
	se.closeOnce.Do(func() { close(se.closeChan) })
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (se *SumologicExtension) validateCredentials(
	ctx context.Context,
	colCreds credentials.CollectorCredentials,
) error {
	se.logger.Info("Validating collector credentials...",
		zap.String(collectorCredentialIDField, colCreds.Credentials.CollectorCredentialID),
		zap.String(collectorIDField, colCreds.Credentials.CollectorID),
	)

	if err := se.injectCredentials(ctx, colCreds); err != nil {
		return err
	}

	se.backOff.Reset()
	var err error

	for {
		err = se.sendHeartbeatWithHTTPClient(ctx, se.httpClient)

		if errors.Is(err, errUnauthorizedHeartbeat) || err == nil {
			return err
		}

		nbo := se.backOff.NextBackOff()
		var backOffErr *backoff.PermanentError
		// Return error if backoff reaches the limit or uncoverable error is spotted
		if ok := errors.As(err, &backOffErr); nbo == se.backOff.Stop || ok {
			return err
		}

		se.logger.Info(fmt.Sprintf("Retrying credentials validation due to error %s", err))

		t := time.NewTimer(nbo)
		defer t.Stop()

		select {
		case <-t.C:
		case <-ctx.Done():
			return fmt.Errorf("credential validation cancelled: %w", ctx.Err())
		}
	}
}

// injectCredentials injects the collector credentials:
//   - into registration info that's stored in the extension and can be used by roundTripper
//   - into http client and its transport so that each request is using collector
//     credentials as authentication keys
func (se *SumologicExtension) injectCredentials(ctx context.Context, colCreds credentials.CollectorCredentials) error {
	se.credsNotifyLock.Lock()
	defer se.credsNotifyLock.Unlock()

	// Set the registration info so that it can be used in RoundTripper.
	se.registrationInfo = colCreds.Credentials

	httpClient, err := se.getHTTPClient(ctx, se.conf.ClientConfig, colCreds.Credentials)
	if err != nil {
		return err
	}

	se.httpClient = httpClient

	// Let components know that the credentials may have changed.
	close(se.credsNotifyUpdate)
	se.credsNotifyUpdate = make(chan struct{})

	return nil
}

func (se *SumologicExtension) getHTTPClient(
	ctx context.Context,
	httpClientSettings confighttp.ClientConfig,
	_ api.OpenRegisterResponsePayload,
) (*http.Client, error) {
	httpClient, err := httpClientSettings.ToClient(
		ctx,
		se.host,
		componenttest.NewNopTelemetrySettings(),
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't create HTTP client: %w", err)
	}

	// Set the transport so that all requests from httpClient will contain
	// the collector credentials.
	httpClient.Transport, err = se.RoundTripper(httpClient.Transport)
	if err != nil {
		return nil, fmt.Errorf("couldn't create HTTP client transport: %w", err)
	}

	return httpClient, nil
}

// getCredentials retrieves the credentials for the collector.
// It does so by checking the local credentials store and by validating those credentials.
// In case they are invalid or are not available through local credentials store
// then it tries to register the collector using the provided access keys.
func (se *SumologicExtension) getCredentials(ctx context.Context) (credentials.CollectorCredentials, error) {
	var (
		colCreds credentials.CollectorCredentials
		err      error
	)

	if !se.conf.ForceRegistration {
		colCreds, err = se.getLocalCredentials(ctx)
		if err == nil {
			errV := se.validateCredentials(ctx, colCreds)

			if errV == nil {
				se.logger.Info("Found stored credentials, skipping registration",
					zap.String(collectorNameField, colCreds.Credentials.CollectorName),
				)
				return colCreds, nil
			}

			// We are unable to confirm if credentials are valid or not as we do not have (clear) response from the API
			if !errors.Is(errV, errUnauthorizedHeartbeat) {
				return credentials.CollectorCredentials{}, errV
			}

			// Credentials might have ended up being invalid or the collector
			// might have been removed in Sumo.
			// Fall back to removing the credentials and recreating them by registering
			// the collector.
			if err = se.credentialsStore.Delete(se.hashKey); err != nil {
				se.logger.Error(
					"Unable to delete old collector credentials", zap.Error(err),
				)
			}

			se.logger.Info("Locally stored credentials invalid. Trying to re-register...",
				zap.String(collectorNameField, colCreds.Credentials.CollectorName),
				zap.String(collectorIDField, colCreds.Credentials.CollectorID),
				zap.Error(errV),
			)
		} else {
			se.logger.Info("Locally stored credentials not found, registering the collector")
		}
	}

	colCreds, err = se.getCredentialsByRegistering(ctx)
	if err != nil {
		return credentials.CollectorCredentials{}, err
	}

	return colCreds, nil
}

// getCredentialsByRegistering registers the collector and returns the credentials
// obtained from the API.
func (se *SumologicExtension) getCredentialsByRegistering(ctx context.Context) (credentials.CollectorCredentials, error) {
	colCreds, err := se.registerCollectorWithBackoff(ctx, se.collectorName)
	if err != nil {
		return credentials.CollectorCredentials{}, err
	}
	if err := se.credentialsStore.Store(se.hashKey, colCreds); err != nil {
		se.logger.Error(
			"Unable to store collector credentials, they will be used now but won't be re-used on next run",
			zap.Error(err),
		)
	}

	se.collectorName = colCreds.CollectorName

	return colCreds, nil
}

// getLocalCredentials returns the credentials retrieved from local credentials
// storage in case they are available there.
func (se *SumologicExtension) getLocalCredentials(_ context.Context) (credentials.CollectorCredentials, error) {
	colCreds, err := se.credentialsStore.Get(se.hashKey)
	if err != nil {
		return credentials.CollectorCredentials{},
			fmt.Errorf("problem finding local collector credentials (hash key: %s): %w",
				se.hashKey, err,
			)
	}

	se.collectorName = colCreds.CollectorName
	if colCreds.APIBaseURL != "" {
		se.SetBaseURL(colCreds.APIBaseURL)
	}

	return colCreds, nil
}

// registerCollector registers the collector using registration API and returns
// the obtained collector credentials.
func (se *SumologicExtension) registerCollector(ctx context.Context, collectorName string) (credentials.CollectorCredentials, error) {
	u, err := url.Parse(se.BaseURL())
	if err != nil {
		return credentials.CollectorCredentials{}, err
	}
	u.Path = registerURL

	hostname, err := getHostname(se.logger)
	if err != nil {
		return credentials.CollectorCredentials{}, fmt.Errorf("cannot get hostname: %w", err)
	}

	var buff bytes.Buffer
	if err = json.NewEncoder(&buff).Encode(api.OpenRegisterRequestPayload{
		CollectorName: collectorName,
		Description:   se.conf.CollectorDescription,
		Category:      se.conf.CollectorCategory,
		Fields:        se.conf.CollectorFields,
		Hostname:      hostname,
		Ephemeral:     se.conf.Ephemeral,
		Clobber:       se.conf.Clobber,
		TimeZone:      se.conf.TimeZone,
	}); err != nil {
		return credentials.CollectorCredentials{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &buff)
	if err != nil {
		return credentials.CollectorCredentials{}, err
	}

	addClientCredentials(req,
		se.conf.Credentials,
	)
	addJSONHeaders(req)

	se.logger.Info("Calling register API", zap.String("URL", u.String()))

	client := *http.DefaultClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	res, err := client.Do(req)
	if err != nil {
		se.logger.Warn("Collector registration HTTP request failed", zap.Error(err))
		return credentials.CollectorCredentials{}, fmt.Errorf("failed to register the collector: %w", err)
	}

	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return credentials.CollectorCredentials{}, se.handleRegistrationError(res)
	} else if res.StatusCode == http.StatusMovedPermanently {
		// Use the URL from Location header for subsequent requests.
		u := strings.TrimSuffix(res.Header.Get("Location"), "/")
		se.SetBaseURL(u)
		se.logger.Info("Redirected to a different deployment",
			zap.String("url", u),
		)
		return se.registerCollector(ctx, collectorName)
	}

	var resp api.OpenRegisterResponsePayload
	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return credentials.CollectorCredentials{}, err
	}

	if collectorName != resp.CollectorName {
		se.logger.Warn("Collector name already in use, registered modified name", zap.String("registered_name", resp.CollectorName))
	}

	return credentials.CollectorCredentials{
		CollectorName: collectorName,
		Credentials:   resp,
		APIBaseURL:    se.BaseURL(),
	}, nil
}

// handleRegistrationError handles the collector registration errors and returns
// appropriate error for backoff handling and logging purposes.
func (se *SumologicExtension) handleRegistrationError(res *http.Response) error {
	var errResponse api.ErrorResponsePayload
	if err := json.NewDecoder(res.Body).Decode(&errResponse); err != nil {
		var buff bytes.Buffer
		if _, errCopy := io.Copy(&buff, res.Body); errCopy != nil {
			return fmt.Errorf(
				"failed to read the collector registration response body, status code: %d, err: %w",
				res.StatusCode, errCopy,
			)
		}
		return fmt.Errorf(
			"failed to decode collector registration response body: %s, status code: %d, err: %w",
			buff.String(), res.StatusCode, err,
		)
	}

	se.logger.Warn("Collector registration failed",
		zap.Int("status_code", res.StatusCode),
		zap.String("error_id", errResponse.ID),
		zap.Any("errors", errResponse.Errors),
	)

	// Return unrecoverable error for 4xx status codes except 429
	if res.StatusCode >= 400 && res.StatusCode < 500 && res.StatusCode != http.StatusTooManyRequests {
		return backoff.Permanent(fmt.Errorf(
			"failed to register the collector, got HTTP status code: %d",
			res.StatusCode,
		))
	}

	return fmt.Errorf(
		"failed to register the collector, got HTTP status code: %d", res.StatusCode,
	)
}

// callRegisterWithBackoff calls registration using exponential backoff algorithm
// this loosely base on backoff.Retry function
func (se *SumologicExtension) registerCollectorWithBackoff(ctx context.Context, collectorName string) (credentials.CollectorCredentials, error) {
	se.backOff.Reset()
	for {
		creds, err := se.registerCollector(ctx, collectorName)
		if err == nil {
			se.logger = se.origLogger.With(
				zap.String(collectorNameField, creds.Credentials.CollectorName),
				zap.String(collectorIDField, creds.Credentials.CollectorID),
			)
			se.logger.Info("Collector registration finished successfully")

			return creds, nil
		}

		nbo := se.backOff.NextBackOff()
		var backOffErr *backoff.PermanentError
		// Return error if backoff reaches the limit or uncoverable error is spotted
		if ok := errors.As(err, &backOffErr); nbo == se.backOff.Stop || ok {
			return credentials.CollectorCredentials{}, fmt.Errorf("collector registration failed: %w", err)
		}

		t := time.NewTimer(nbo)
		defer t.Stop()

		select {
		case <-t.C:
		case <-ctx.Done():
			return credentials.CollectorCredentials{}, fmt.Errorf("collector registration cancelled: %w", ctx.Err())
		}
	}
}

func (se *SumologicExtension) heartbeatLoop() {
	if se.registrationInfo.CollectorCredentialID == "" || se.registrationInfo.CollectorCredentialKey == "" {
		se.logger.Error("Collector not registered, cannot send heartbeat")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// When the close channel is closed ...
		<-se.closeChan
		// ... cancel the ongoing heartbeat request.
		cancel()
	}()

	se.logger.Info("Heartbeat loop initialized. Starting to send heartbeat requests")
	timer := time.NewTimer(se.conf.HeartBeatInterval)
	for {
		select {
		case <-se.closeChan:
			se.logger.Info("Heartbeat sender turned off")
			return

		default:
			err := se.sendHeartbeatWithHTTPClient(ctx, se.httpClient)

			if err != nil {
				if errors.Is(err, errUnauthorizedHeartbeat) {
					se.logger.Warn("Heartbeat request unauthorized, re-registering the collector")
					var colCreds credentials.CollectorCredentials
					colCreds, err = se.getCredentialsByRegistering(ctx)
					if err != nil {
						se.logger.Error("Heartbeat error, cannot register the collector", zap.Error(err))
						continue
					}

					// Inject newly received credentials into extension's configuration.
					if err = se.injectCredentials(ctx, colCreds); err != nil {
						se.logger.Error("Heartbeat error, cannot inject new collector credentials", zap.Error(err))
						continue
					}

					// Overwrite old logger fields with new collector name and ID.
					se.logger = se.origLogger.With(
						zap.String(collectorNameField, colCreds.Credentials.CollectorName),
						zap.String(collectorIDField, colCreds.Credentials.CollectorID),
					)
				} else {
					se.logger.Error("Heartbeat error", zap.Error(err))
				}
			} else {
				se.logger.Debug("Heartbeat sent")
			}

			select {
			case <-timer.C:
				timer.Stop()
				timer.Reset(se.conf.HeartBeatInterval)
			case <-se.closeChan:
			}
		}
	}
}

var (
	errUnauthorizedHeartbeat = errors.New("heartbeat unauthorized")
	errUnauthorizedMetadata  = errors.New("metadata update unauthorized")
)

type ErrorAPI struct {
	status int
	body   string
}

func (e ErrorAPI) Error() string {
	return fmt.Sprintf("API error (status code: %d): %s", e.status, e.body)
}

func (se *SumologicExtension) sendHeartbeatWithHTTPClient(ctx context.Context, httpClient *http.Client) error {
	u, err := url.Parse(se.BaseURL() + heartbeatURL)
	if err != nil {
		return fmt.Errorf("unable to parse heartbeat URL %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("unable to create HTTP request %w", err)
	}

	addJSONHeaders(req)
	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to send HTTP request: %w", err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	default:
		var buff bytes.Buffer

		if _, err := io.Copy(&buff, res.Body); err != nil {
			return fmt.Errorf(
				"failed to copy collector heartbeat response body, status code: %d, err: %w",
				res.StatusCode, err,
			)
		}

		return fmt.Errorf("collector heartbeat request failed: %w",
			ErrorAPI{
				status: res.StatusCode,
				body:   buff.String(),
			},
		)

	case http.StatusUnauthorized:
		return errUnauthorizedHeartbeat

	case http.StatusNoContent:
	}

	return nil
}

func baseURL() (string, error) {
	// This doesn't connect, we just need the connection object.
	c, err := net.Dial("udp", "255.255.255.255:53")
	if err != nil {
		return "", err
	}

	defer c.Close()
	a := c.LocalAddr().(*net.UDPAddr)
	h, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return "", err
	}

	return h, nil
}

var sumoAppProcesses = map[string]string{
	"apache":                "apache",
	"apache2":               "apache",
	"httpd":                 "apache",
	"docker":                "docker", // docker cli
	"elasticsearch":         "elasticsearch",
	"mysql-server":          "mysql",
	"mysqld":                "mysql",
	"nginx":                 "nginx",
	"postgresql":            "postgres",
	"postgresql-9.5":        "postgres",
	"rabbitmq-server":       "rabbitmq",
	"redis":                 "redis",
	"tomcat":                "tomcat",
	"kafka-server-start.sh": "kafka", // Need to test this, most common shell wrapper.
	"redis-server":          "redis",
	"mongod":                "mongodb",
	"cassandra":             "cassandra",
	"jmx":                   "jmx",
	"activemq":              "activemq",
	"memcached":             "memcached",
	"haproxy":               "haproxy",
	"dockerd":               "docker-ce", // docker engine, for when process runs natively
	"com.docker.backend":    "docker-ce", // docker daemon runs on a VM in Docker Desktop, process doesn't show on mac
	"sqlservr":              "mssql",     // linux SQL Server process
}

func (se *SumologicExtension) filteredProcessList() ([]string, error) {
	var pl []string

	processes, err := process.Processes()
	if err != nil {
		return pl, err
	}

	for _, v := range processes {
		e, err := v.Name()
		if err != nil {
			if runtime.GOOS == "windows" {
				// On Windows, if we can't get a process name, it is likely a zombie process, assume that and skip them.
				se.logger.Warn(
					"Failed to get executable name, it is likely a zombie process, skipping it",
					zap.Int32("pid", v.Pid),
					zap.Error(err))
				continue
			}

			return nil, fmt.Errorf("Error getting executable name: %w", err)
		}
		e = strings.ToLower(e)

		if a, i := sumoAppProcesses[e]; i {
			pl = append(pl, a)
		}

		// handling for Docker Desktop
		if e == dockerDesktopJavaProcess {
			pl = append(pl, "docker-ce")
		}

		// handling Java background processes
		if e == "java" {
			cmdline, err := v.Cmdline()
			if err != nil {
				return nil, fmt.Errorf("error getting executable name for PID %d: %w", v.Pid, err)
			}

			switch {
			case strings.Contains(cmdline, cassandraJavaProcess):
				pl = append(pl, "cassandra")
			case strings.Contains(cmdline, jmxJavaProcess):
				pl = append(pl, "jmx")
			case strings.Contains(cmdline, activeMQJavaProcess):
				pl = append(pl, "activemq")
			}
		}
	}

	return pl, nil
}

func (se *SumologicExtension) discoverTags() (map[string]any, error) {
	t := map[string]any{
		"sumo.disco.enabled": "true",
	}

	pl, err := se.filteredProcessList()
	if err != nil {
		return t, err
	}

	for _, v := range pl {
		t["sumo.disco."+v] = 1 // Sumo does not allow empty tag values, let's set it to anything.
	}

	return t, nil
}

func (se *SumologicExtension) updateMetadataWithHTTPClient(ctx context.Context, httpClient *http.Client) error {
	u, err := url.Parse(se.BaseURL() + metadataURL)
	if err != nil {
		return fmt.Errorf("unable to parse metadata URL %w", err)
	}

	info, err := host.Info()
	if err != nil {
		return err
	}

	hostname, err := getHostname(se.logger)
	if err != nil {
		return err
	}

	ip, err := baseURL()
	if err != nil {
		return err
	}

	td := map[string]any{}

	if se.conf.DiscoverCollectorTags {
		td, err = se.discoverTags()
		if err != nil {
			return err
		}
	}

	for k, v := range se.conf.CollectorFields {
		td[k] = v
	}

	var buff bytes.Buffer
	if err = json.NewEncoder(&buff).Encode(api.OpenMetadataRequestPayload{
		HostDetails: api.OpenMetadataHostDetails{
			Name:        hostname,
			OsName:      info.OS,
			OsVersion:   info.PlatformVersion,
			Environment: se.conf.CollectorEnvironment,
		},
		CollectorDetails: api.OpenMetadataCollectorDetails{
			RunningVersion: cleanupBuildVersion(se.buildVersion),
		},
		NetworkDetails: api.OpenMetadataNetworkDetails{
			HostIPAddress: ip,
		},
		TagDetails: td,
	}); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), &buff)
	if err != nil {
		return fmt.Errorf("unable to create HTTP request %w", err)
	}

	addJSONHeaders(req)

	se.logger.Info("Updating collector metadata",
		zap.String("URL", u.String()),
		zap.String("body", buff.String()))

	res, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to send HTTP request: %w", err)
	}
	defer res.Body.Close()

	switch res.StatusCode {
	default:
		var buff bytes.Buffer
		if _, err := io.Copy(&buff, res.Body); err != nil {
			return fmt.Errorf(
				"failed to copy collector metadata response body, status code: %d, err: %w",
				res.StatusCode, err,
			)
		}

		se.logger.Warn("Metadata API error response",
			zap.Int("status", res.StatusCode),
			zap.String("body", buff.String()))

		return fmt.Errorf("collector metadata request failed: %w",
			ErrorAPI{
				status: res.StatusCode,
				body:   buff.String(),
			},
		)

	case http.StatusUnauthorized:
		return errUnauthorizedMetadata
	case http.StatusNoContent:
	case http.StatusOK:
	}

	return nil
}

func (se *SumologicExtension) updateMetadataWithBackoff(ctx context.Context) error {
	se.backOff.Reset()
	for {
		err := se.updateMetadataWithHTTPClient(ctx, se.httpClient)
		if err == nil {
			return nil
		}

		se.logger.Warn(fmt.Sprintf("collector metadata update failed: %s", err))

		nbo := se.backOff.NextBackOff()
		var backOffErr *backoff.PermanentError
		// Return error if backoff reaches the limit or uncoverable error is spotted
		if ok := errors.As(err, &backOffErr); nbo == se.backOff.Stop || ok {
			return fmt.Errorf("collector metadata update failed: %w", err)
		}

		t := time.NewTimer(nbo)
		defer t.Stop()

		select {
		case <-t.C:
		case <-ctx.Done():
			return fmt.Errorf("collector metadata update cancelled: %w", ctx.Err())
		}
	}
}

func (se *SumologicExtension) ComponentID() component.ID {
	return se.id
}

func (se *SumologicExtension) CollectorID() string {
	return se.registrationInfo.CollectorID
}

func (se *SumologicExtension) BaseURL() string {
	se.baseURLLock.RLock()
	defer se.baseURLLock.RUnlock()
	return se.baseURL
}

func (se *SumologicExtension) SetBaseURL(baseURL string) {
	se.baseURLLock.Lock()
	se.baseURL = baseURL
	se.baseURLLock.Unlock()
}

func (se *SumologicExtension) StickySessionCookie() string {
	se.stickySessionCookieLock.RLock()
	defer se.stickySessionCookieLock.RUnlock()
	return se.stickySessionCookie
}

func (se *SumologicExtension) SetStickySessionCookie(stickySessionCookie string) {
	se.stickySessionCookieLock.Lock()
	se.stickySessionCookie = stickySessionCookie
	se.stickySessionCookieLock.Unlock()
}

// WatchCredentialKey watches for credential key updates. It makes use of a
// channel close (done by injectCredentials) and string comparison with a
// known/previous credential key (old). This function allows components to be
// proactive when dealing with changes to authentication.
func (se *SumologicExtension) WatchCredentialKey(ctx context.Context, old string) string {
	se.credsNotifyLock.Lock()
	v, ch := se.registrationInfo.CollectorCredentialKey, se.credsNotifyUpdate
	se.credsNotifyLock.Unlock()

	for v == old {
		select {
		case <-ctx.Done():
			return v
		case <-ch:
			se.credsNotifyLock.Lock()
			v, ch = se.registrationInfo.CollectorCredentialKey, se.credsNotifyUpdate
			se.credsNotifyLock.Unlock()
		}
	}

	return v
}

// CreateCredentialsHeader produces an HTTP header containing authentication
// credentials. This function is for components that do not make use of the
// RoundTripper or have an HTTP request to build upon.
func (se *SumologicExtension) CreateCredentialsHeader() (http.Header, error) {
	id, key := se.registrationInfo.CollectorCredentialID, se.registrationInfo.CollectorCredentialKey

	if id == "" || key == "" {
		return nil, errors.New("collector credentials are not set")
	}

	token := base64.StdEncoding.EncodeToString(
		[]byte(id + ":" + key),
	)

	header := http.Header{}
	header.Set("Authorization", "Basic "+token)

	return header, nil
}

// Implement [1] in order for this extension to be used as custom exporter
// authenticator.
//
// [1]: https://github.com/open-telemetry/opentelemetry-collector/blob/2e84285efc665798d76773b9901727e8836e9d8f/config/configauth/clientauth.go#L34-L39
func (se *SumologicExtension) RoundTripper(base http.RoundTripper) (http.RoundTripper, error) {
	return roundTripper{
		collectorCredentialID:     se.registrationInfo.CollectorCredentialID,
		collectorCredentialKey:    se.registrationInfo.CollectorCredentialKey,
		addStickySessionCookie:    se.addStickySessionCookie,
		updateStickySessionCookie: se.updateStickySessionCookie,
		base:                      base,
	}, nil
}

func (se *SumologicExtension) addStickySessionCookie(req *http.Request) {
	if !se.conf.StickySessionEnabled {
		return
	}
	currentCookieValue := se.StickySessionCookie()
	if currentCookieValue != "" {
		cookie := &http.Cookie{
			Name:  stickySessionKey,
			Value: currentCookieValue,
		}
		req.AddCookie(cookie)
	}
}

func (se *SumologicExtension) updateStickySessionCookie(resp *http.Response) {
	cookies := resp.Cookies()
	if se.conf.StickySessionEnabled && len(cookies) > 0 {
		for _, cookie := range cookies {
			if cookie.Name == stickySessionKey {
				if cookie.Value != se.StickySessionCookie() {
					se.SetStickySessionCookie(cookie.Value)
				}
				return
			}
		}
	}
}

type roundTripper struct {
	collectorCredentialID     string
	collectorCredentialKey    string
	addStickySessionCookie    func(*http.Request)
	updateStickySessionCookie func(*http.Response)
	base                      http.RoundTripper
}

func (rt roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	addCollectorCredentials(req, rt.collectorCredentialID, rt.collectorCredentialKey)
	rt.addStickySessionCookie(req)
	resp, err := rt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	rt.updateStickySessionCookie(resp)
	return resp, err
}

func addCollectorCredentials(req *http.Request, collectorCredentialID string, collectorCredentialKey string) {
	token := base64.StdEncoding.EncodeToString(
		[]byte(collectorCredentialID + ":" + collectorCredentialKey),
	)

	// Delete the existing Authorization header so prevent sending both the old one
	// and the new one.
	req.Header.Del("Authorization")
	req.Header.Add("Authorization", "Basic "+token)
}

func addClientCredentials(req *http.Request, credentials accessCredentials) {
	var authHeaderValue string
	if credentials.InstallationToken != "" {
		authHeaderValue = fmt.Sprintf("Bearer %s", string(credentials.InstallationToken))
	}

	req.Header.Del("Authorization")
	req.Header.Add("Authorization", authHeaderValue)
}

// TODO(ck): hostname allows the darwin tests to bypass fqdn.
var hostname = fqdn.FqdnHostname

// getHostname returns the host name consistently with the resource detection processor's defaults
// TODO: try to dynamically extract this from the resource processor in the pipeline
func getHostname(logger *zap.Logger) (string, error) {
	fqdnHostname, err := hostname()
	if err == nil {
		return fqdnHostname, nil
	}
	logger.Debug("failed to get fqdn", zap.Error(err))

	return os.Hostname()
}

// cleanupBuildVersion adds a leading 'v' and removes the tailing build hash to make sure the
// backend understand the build number. Note that only version strings with the following format will be
// cleaned up. All other version formats will remain the same.
// Cleaned up format: 0.108.0-sumo-2-4d57200692d5c5c39effad4ae3b29fef79209113
func cleanupBuildVersion(version string) string {
	pattern := "^v?([0-9]+\\.[0-9]+\\.[0-9]+-sumo-[0-9]+)(-[0-9a-f]{40}){0,1}(-fips){0,1}$"
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(version, 1)
	if len(matches) != 1 {
		return version
	}
	subMatches := matches[0]
	if len(subMatches) > 1 {
		ver := subMatches[1]
		if len(subMatches) == 4 {
			ver += subMatches[3]
		}
		return "v" + ver
	}

	return version
}
