//go:build integration

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package testcontainers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/client"
	derrdefs "github.com/docker/docker/errdefs"
	"github.com/moby/moby/api/types/container"
	dockernetwork "github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"software.sslmate.com/src/go-pkcs12"
)

const (
	kafkaImage    = "confluentinc/cp-kafka:7.6.1"
	keycloakImage = "quay.io/keycloak/keycloak:26.3.3"

	keystorePassword = "changeit"
	realmName        = "otel"
)

// RequiredImages returns the container images required for the Kafka OAuth integration environment.
func RequiredImages() []string {
	return []string{kafkaImage, keycloakImage}
}

// CheckImagesPresent checks whether the required images are present locally.
// It does not pull images.
func CheckImagesPresent(ctx context.Context) (missing []string, err error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("initialize docker client: %w", err)
	}

	// Fail early if the daemon is unreachable, to avoid returning misleading "missing image" results.
	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker daemon unreachable: %w", err)
	}

	for _, image := range RequiredImages() {
		_, _, err := cli.ImageInspectWithRaw(ctx, image)
		if err == nil {
			continue
		}
		if derrdefs.IsNotFound(err) {
			missing = append(missing, image)
			continue
		}
		return nil, fmt.Errorf("inspect docker image %q: %w", image, err)
	}
	return missing, nil
}

// Environment encapsulates the Kafka + Keycloak resources required to run
// OAuth integration tests.
// Close must be called to tear down containers and temporary assets.
type Environment struct {
	network         testcontainers.Network
	kafka           testcontainers.Container
	keycloak        testcontainers.Container
	tempDir         string
	tokenTTLSeconds int

	Bootstrap    string
	TokenURL     string
	IssuerURL    string
	JWKSURL      string
	ClientID     string
	ClientSecret string
	// RevokedClientID/RevokedClientSecret identify a second Keycloak client that is
	// provisioned in a revoked/disabled state for deterministic negative-path tests.
	RevokedClientID     string
	RevokedClientSecret string

	CACert     []byte
	CACertPath string

	TLS configtls.ClientConfig
}

// Close releases all resources associated with the environment.
func (e *Environment) Close(ctx context.Context) error {
	var errs []error

	if e.kafka != nil {
		if err := e.kafka.Terminate(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if e.keycloak != nil {
		if err := e.keycloak.Terminate(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if e.network != nil {
		if err := e.network.Remove(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if e.tempDir != "" {
		if err := os.RemoveAll(e.tempDir); err != nil {
			errs = append(errs, err)
		}
	}

	return errorsJoin(errs...)
}

// NewEnvironment spins up a Kafka + Keycloak deployment configured for
// SASL/OAUTHBEARER with TLS everywhere.
func NewEnvironment(ctx context.Context) (*Environment, error) {
	return NewEnvironmentWithTTL(ctx, 0)
}

// NewEnvironmentWithTTL spins up the environment with a custom access token TTL
// for the Keycloak realm. When ttl is 0, the realm default is used.
func NewEnvironmentWithTTL(ctx context.Context, ttl time.Duration) (*Environment, error) {
	tempDir, err := os.MkdirTemp("", "kafka-oauth-env-")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	env := &Environment{
		tempDir:             tempDir,
		ClientID:            "otel-collector",
		ClientSecret:        "otel-collector-secret",
		RevokedClientID:     "otel-collector-revoked",
		RevokedClientSecret: "otel-collector-revoked-secret",
	}
	if ttl > 0 {
		// Keycloak expects whole seconds
		env.tokenTTLSeconds = int(ttl / time.Second)
	}

	cleanup := func(initErr error) (*Environment, error) {
		cleanupErr := env.Close(ctx)
		if cleanupErr != nil {
			return nil, errorsJoin(initErr, cleanupErr)
		}
		return nil, initErr
	}

	caCert, caKey, err := generateCA()
	if err != nil {
		return cleanup(fmt.Errorf("generate ca: %w", err))
	}
	env.CACert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})

	certDir := filepath.Join(tempDir, "certs")
	if err := os.Mkdir(certDir, 0o755); err != nil {
		return cleanup(fmt.Errorf("create cert dir: %w", err))
	}

	env.CACertPath = filepath.Join(certDir, "ca.crt")
	if err := os.WriteFile(env.CACertPath, env.CACert, 0o644); err != nil {
		return cleanup(fmt.Errorf("write ca cert: %w", err))
	}

	kafkaKey, kafkaCert, err := generateServerCert(caCert, caKey, "kafka", []string{"kafka"})
	if err != nil {
		return cleanup(fmt.Errorf("generate kafka cert: %w", err))
	}
	if err := writePKCS12(certDir, "cluster.keystore.p12", kafkaKey, kafkaCert, caCert); err != nil {
		return cleanup(fmt.Errorf("write kafka keystore: %w", err))
	}
	if err := writeTrustStore(certDir, "cluster.truststore.p12", caCert); err != nil {
		return cleanup(fmt.Errorf("write kafka truststore: %w", err))
	}

	keycloakKey, keycloakCert, err := generateServerCert(caCert, caKey, "keycloak", []string{"keycloak"})
	if err != nil {
		return cleanup(fmt.Errorf("generate keycloak cert: %w", err))
	}
	keycloakCertDir := filepath.Join(tempDir, "keycloak-certs")
	if err := os.Mkdir(keycloakCertDir, 0o755); err != nil {
		return cleanup(fmt.Errorf("create keycloak cert dir: %w", err))
	}
	if err := writePKCS12(keycloakCertDir, "keycloak.p12", keycloakKey, keycloakCert, caCert); err != nil {
		return cleanup(fmt.Errorf("write keycloak keystore: %w", err))
	}

	realmDir := filepath.Join(tempDir, "realm")
	if err := os.Mkdir(realmDir, 0o755); err != nil {
		return cleanup(fmt.Errorf("create realm dir: %w", err))
	}
	realmPath := filepath.Join(realmDir, "realm.json")
	if err := os.WriteFile(realmPath, []byte(realmJSON(env.ClientID, env.ClientSecret, env.RevokedClientID, env.RevokedClientSecret, env.tokenTTLSeconds)), 0o644); err != nil {
		return cleanup(fmt.Errorf("write realm json: %w", err))
	}

	networkName := fmt.Sprintf("kafka-oauth-%s", randomName())
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{Name: networkName},
	})
	if err != nil {
		return cleanup(fmt.Errorf("create network: %w", err))
	}
	env.network = network

	// Choose a host port for Keycloak (prefer 18443, fallback to any free port).
	keycloakHost := "127.0.0.1"
	keycloakHostPort := "18443"
	if !isHostPortFree(keycloakHost, keycloakHostPort) {
		keycloakHostPort = findFreeHostPort(keycloakHost)
	}

	keycloakReq := testcontainers.ContainerRequest{
		Image:        keycloakImage,
		ExposedPorts: []string{"8080/tcp", "8443/tcp"},
		Env: map[string]string{
			"KEYCLOAK_ADMIN":          "admin",
			"KEYCLOAK_ADMIN_PASSWORD": "admin",
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = dockernetwork.PortMap{
				dockernetwork.MustParsePort("8443/tcp"): {
					{HostIP: netip.MustParseAddr(keycloakHost), HostPort: keycloakHostPort},
				},
			}
		},
		Cmd: []string{
			"start",
			"--import-realm",
			"--https-key-store-file=/opt/keycloak/data/certs/keycloak.p12",
			fmt.Sprintf("--https-key-store-password=%s", keystorePassword),
			"--http-enabled=true",
			"--hostname-strict=false",
			"--hostname=keycloak",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"keycloak"},
		},
		WaitingFor: wait.ForListeningPort("8443/tcp").WithStartupTimeout(60 * time.Second),
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(keycloakCertDir, testcontainers.ContainerMountTarget("/opt/keycloak/data/certs")),
			testcontainers.BindMount(realmDir, testcontainers.ContainerMountTarget("/opt/keycloak/data/import")),
		),
	}

	keycloakContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: keycloakReq,
		Started:          true,
	})
	if err != nil {
		return cleanup(fmt.Errorf("start keycloak: %w", err))
	}
	env.keycloak = keycloakContainer

	// Two-stage selection for Kafka host port: prefer 19094, else pick a free one.
	kafkaHost := "127.0.0.1"
	kafkaHostPort := "19094"
	if !isHostPortFree(kafkaHost, kafkaHostPort) {
		kafkaHostPort = findFreeHostPort(kafkaHost)
	}

	var kafkaContainer testcontainers.Container
	// Retry a few times in case of race on chosen port.
	for attempt := 0; attempt < 3; attempt++ {
		kafkaReq := testcontainers.ContainerRequest{
			Image:        kafkaImage,
			ExposedPorts: []string{"9094/tcp"},
			Env:          kafkaEnv(kafkaHostPort),
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.PortBindings = dockernetwork.PortMap{
					dockernetwork.MustParsePort("9094/tcp"): {
						{HostIP: netip.MustParseAddr(kafkaHost), HostPort: kafkaHostPort},
					},
				}
			},
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"kafka"},
			},
			WaitingFor: wait.ForExec([]string{"bash", "-c", "test -f /var/lib/kafka/data/meta.properties"}).WithStartupTimeout(60 * time.Second),
			Mounts: testcontainers.Mounts(
				testcontainers.BindMount(certDir, testcontainers.ContainerMountTarget("/etc/kafka/secrets")),
			),
		}
		kafkaContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: kafkaReq,
			Started:          true,
		})
		if err == nil {
			break
		}
		if !strings.Contains(strings.ToLower(err.Error()), "port is already allocated") {
			return cleanup(fmt.Errorf("start kafka: %w", err))
		}
		// pick another free port and retry
		kafkaHostPort = findFreeHostPort(kafkaHost)
	}
	if err != nil {
		return cleanup(fmt.Errorf("start kafka: %w", err))
	}
	env.kafka = kafkaContainer

	host, err := kafkaContainer.Host(ctx)
	if err != nil {
		return cleanup(fmt.Errorf("kafka host: %w", err))
	}
	mappedPort, err := kafkaContainer.MappedPort(ctx, "9094/tcp")
	if err != nil {
		return cleanup(fmt.Errorf("kafka mapped port: %w", err))
	}
	env.Bootstrap = net.JoinHostPort(host, mappedPort.Port())

	keycloakPort, err := keycloakContainer.MappedPort(ctx, "8443/tcp")
	if err != nil {
		return cleanup(fmt.Errorf("keycloak mapped port: %w", err))
	}

	// Use the same explicit IPv4 loopback host we bound to above. This avoids
	// variability from "localhost" resolving to IPv6 (::1) on some systems and
	// keeps the token endpoint consistent with the port binding.
	authority := net.JoinHostPort(keycloakHost, keycloakPort.Port())
	env.TokenURL = fmt.Sprintf("https://%s/realms/%s/protocol/openid-connect/token", authority, realmName)
	env.IssuerURL = fmt.Sprintf("https://%s/realms/%s", authority, realmName)
	env.JWKSURL = fmt.Sprintf("https://%s/realms/%s/protocol/openid-connect/certs", authority, realmName)

	env.CACertPath = filepath.Join(certDir, "ca.crt")
	env.TLS = configtls.ClientConfig{}
	env.TLS.CAPem = configopaque.String(string(env.CACert))
	env.TLS.ServerName = "kafka"

	// Explicit readiness probes to avoid flakiness from "port is listening" but not actually usable.
	if err := env.waitUntilReady(ctx); err != nil {
		return cleanup(err)
	}

	return env, nil
}

func (e *Environment) waitUntilReady(ctx context.Context) error {
	if err := e.waitForKeycloakTokenEndpoint(ctx); err != nil {
		return err
	}
	if err := e.waitForKafkaBroker(ctx); err != nil {
		return err
	}
	return nil
}

func (e *Environment) waitForKeycloakTokenEndpoint(ctx context.Context) error {
	deadline, _ := ctx.Deadline()
	var lastErr error
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		_, _, err := e.RequestClientToken(ctx, e.ClientID, e.ClientSecret)
		if err == nil {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return fmt.Errorf("keycloak not ready (token endpoint) by %s: %w\n--- keycloak logs (tail) ---\n%s",
				deadline.Format(time.RFC3339), lastErr, e.containerLogs(ctx, e.keycloak))
		case <-ticker.C:
		}
	}
}

func (e *Environment) waitForKafkaBroker(ctx context.Context) error {
	deadline, _ := ctx.Deadline()
	var lastErr error
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		// Use internal PLAINTEXT listener; validates the broker is actually serving requests.
		exitCode, _, err := e.kafka.Exec(ctx, []string{"bash", "-c", "kafka-broker-api-versions --bootstrap-server kafka:9092 >/dev/null 2>&1"})
		if err == nil && exitCode == 0 {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("kafka-broker-api-versions exit code %d", exitCode)
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("kafka not ready (broker api) by %s: %w\n--- kafka logs (tail) ---\n%s",
				deadline.Format(time.RFC3339), lastErr, e.containerLogs(ctx, e.kafka))
		case <-ticker.C:
		}
	}
}

func (e *Environment) containerLogs(ctx context.Context, c testcontainers.Container) string {
	if c == nil {
		return ""
	}
	r, err := c.Logs(ctx)
	if err != nil {
		return fmt.Sprintf("failed to read logs: %v", err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return fmt.Sprintf("failed to read logs: %v", err)
	}
	const max = 16_000
	if len(b) <= max {
		return string(b)
	}
	return string(b[len(b)-max:])
}

// RequestClientToken performs a client-credentials token request against the environment's Keycloak.
// It returns (accessToken, expiresInSeconds, error).
func (e *Environment) RequestClientToken(ctx context.Context, clientID, clientSecret string) (string, int, error) {
	cp := x509.NewCertPool()
	if ok := cp.AppendCertsFromPEM(e.CACert); !ok {
		return "", 0, errors.New("failed to append CA cert")
	}
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    cp,
				ServerName: "keycloak",
			},
		},
	}

	form := url.Values{}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("grant_type", "client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", 0, fmt.Errorf("token request failed: status=%s body=%s", resp.Status, strings.TrimSpace(string(body)))
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("decode token response: %w", err)
	}
	if tokenResp.AccessToken == "" {
		return "", 0, errors.New("empty access_token in token response")
	}
	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// isHostPortFree checks if host:port is available for binding.
func isHostPortFree(host, port string) bool {
	l, err := net.Listen("tcp", net.JoinHostPort(host, port))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// findFreeHostPort returns a free TCP port on host by momentarily binding to :0.
func findFreeHostPort(host string) string {
	l, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		// Fallback to common ephemeral range start if :0 fails (very unlikely)
		return "49152"
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return strconv.Itoa(addr.Port)
}

func kafkaEnv(externalPort string) map[string]string {
	jwksURL := fmt.Sprintf("https://keycloak:8443/realms/%s/protocol/openid-connect/certs", realmName)
	jaasConfig := "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required ;"
	// Force periodic re-authentication on the external SASL listener to enable deterministic refresh testing.
	// connections.max.reauth.ms sets the maximum time a connection can be idle before requiring re-authentication.
	// Using 3000ms (3 seconds) provides a fast but not brittle interval for refresh tests.
	maxReauthMS := "3000"
	return map[string]string{
		"KAFKA_ENABLE_KRAFT":                                                          "yes",
		"KAFKA_CLUSTER_ID":                                                            "LKc1U7yqStu6wETTy8Hh3g",
		"CLUSTER_ID":                                                                  "LKc1U7yqStu6wETTy8Hh3g",
		"KAFKA_PROCESS_ROLES":                                                         "broker,controller",
		"KAFKA_NODE_ID":                                                               "1",
		"KAFKA_CONTROLLER_LISTENER_NAMES":                                             "CONTROLLER",
		"KAFKA_CONTROLLER_QUORUM_VOTERS":                                              "1@kafka:9093",
		"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":                                        "CONTROLLER:PLAINTEXT,INTERNAL:PLAINTEXT,EXTERNAL:SASL_SSL",
		"KAFKA_LISTENERS":                                                             "CONTROLLER://kafka:9093,INTERNAL://kafka:9092,EXTERNAL://0.0.0.0:9094",
		"KAFKA_ADVERTISED_LISTENERS":                                                  "INTERNAL://kafka:9092,EXTERNAL://localhost:" + externalPort,
		"KAFKA_INTER_BROKER_LISTENER_NAME":                                            "INTERNAL",
		"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":                                      "1",
		"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR":                              "1",
		"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":                                         "1",
		"KAFKA_GROUP_INITIAL_REBALANCE_DELAY_MS":                                      "0",
		"KAFKA_AUTO_CREATE_TOPICS_ENABLE":                                             "true",
		"KAFKA_SUPER_USERS":                                                           "User:service-account-kafka-broker",
		"KAFKA_SSL_KEYSTORE_LOCATION":                                                 "/etc/kafka/secrets/cluster.keystore.p12",
		"KAFKA_SSL_KEYSTORE_PASSWORD":                                                 keystorePassword,
		"KAFKA_SSL_KEYSTORE_TYPE":                                                     "PKCS12",
		"KAFKA_SSL_TRUSTSTORE_LOCATION":                                               "/etc/kafka/secrets/cluster.truststore.p12",
		"KAFKA_SSL_TRUSTSTORE_PASSWORD":                                               keystorePassword,
		"KAFKA_SSL_TRUSTSTORE_TYPE":                                                   "PKCS12",
		"KAFKA_LISTENER_NAME_EXTERNAL_SSL_KEYSTORE_LOCATION":                          "/etc/kafka/secrets/cluster.keystore.p12",
		"KAFKA_LISTENER_NAME_EXTERNAL_SSL_KEYSTORE_PASSWORD":                          keystorePassword,
		"KAFKA_LISTENER_NAME_EXTERNAL_SSL_TRUSTSTORE_LOCATION":                        "/etc/kafka/secrets/cluster.truststore.p12",
		"KAFKA_LISTENER_NAME_EXTERNAL_SSL_TRUSTSTORE_PASSWORD":                        keystorePassword,
		"KAFKA_SASL_ENABLED_MECHANISMS":                                               "OAUTHBEARER",
		"KAFKA_LISTENER_NAME_EXTERNAL_SASL_ENABLED_MECHANISMS":                        "OAUTHBEARER",
		"KAFKA_SASL_OAUTHBEARER_JWKS_ENDPOINT_URL":                                    jwksURL,
		"KAFKA_LISTENER_NAME_EXTERNAL_OAUTHBEARER_SASL_JAAS_CONFIG":                   jaasConfig,
		"KAFKA_LISTENER_NAME_EXTERNAL_OAUTHBEARER_SASL_SERVER_CALLBACK_HANDLER_CLASS": "org.apache.kafka.common.security.oauthbearer.secured.OAuthBearerValidatorCallbackHandler",
		"KAFKA_LISTENER_NAME_EXTERNAL_OAUTHBEARER_SASL_JWKS_ENDPOINT_URL":             jwksURL,
		"KAFKA_LISTENER_NAME_EXTERNAL_CONNECTIONS_MAX_REAUTH_MS":                      maxReauthMS,
		"KAFKA_OPTS": "-Djavax.net.ssl.trustStore=/etc/kafka/secrets/cluster.truststore.p12 -Djavax.net.ssl.trustStorePassword=" + keystorePassword + " -Djavax.net.ssl.trustStoreType=PKCS12",
	}
}

func generateCA() (*x509.Certificate, *rsa.PrivateKey, error) {
	// 2048-bit RSA is plenty for test-only TLS and is significantly faster than 4096-bit.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "Kafka OAuth Test CA",
			Organization: []string{"OpenTelemetry"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func generateServerCert(caCert *x509.Certificate, caKey *rsa.PrivateKey, commonName string, dnsNames []string) (*rsa.PrivateKey, *x509.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: []string{"OpenTelemetry"},
		},
		DNSNames:              dnsNames,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}

func writePKCS12(dir, name string, key *rsa.PrivateKey, cert *x509.Certificate, caCert *x509.Certificate) error {
	pfx, err := pkcs12.Encode(rand.Reader, key, cert, []*x509.Certificate{caCert}, keystorePassword)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), pfx, 0o644)
}

func writeTrustStore(dir, name string, caCert *x509.Certificate) error {
	pfx, err := pkcs12.EncodeTrustStore(rand.Reader, []*x509.Certificate{caCert}, keystorePassword)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), pfx, 0o644)
}

func randomName() string {
	raw, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 32))
	if err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return strings.ToLower(fmt.Sprintf("%x", raw.Uint64()))
}

func realmJSON(clientID, clientSecret, revokedClientID, revokedClientSecret string, tokenTTLSeconds int) string {
	ttlLine := ""
	if tokenTTLSeconds > 0 {
		ttlLine = fmt.Sprintf(`,
  "accessTokenLifespan": %s`, strconv.Itoa(tokenTTLSeconds))
	}
	return fmt.Sprintf(`{
  "realm": "%s",
  "enabled": true,
  "attributes": {
    "frontendUrl": "https://keycloak:8443/realms/%s"
  },
  "sslRequired": "external"%s,
  "clients": [
    {
      "clientId": "kafka-broker",
      "enabled": true,
      "publicClient": false,
      "secret": "kafka-broker-secret",
      "directAccessGrantsEnabled": true,
      "serviceAccountsEnabled": true,
      "standardFlowEnabled": false,
      "implicitFlowEnabled": false
    },
    {
      "clientId": "%s",
      "enabled": true,
      "publicClient": false,
      "secret": "%s",
      "serviceAccountsEnabled": true,
      "directAccessGrantsEnabled": true,
      "standardFlowEnabled": false,
      "implicitFlowEnabled": false
    },
    {
      "clientId": "%s",
      "enabled": false,
      "publicClient": false,
      "secret": "%s",
      "serviceAccountsEnabled": true,
      "directAccessGrantsEnabled": true,
      "standardFlowEnabled": false,
      "implicitFlowEnabled": false
    }
  ],
  "users": [
    {"username": "service-account-kafka-broker", "serviceAccountClientId": "kafka-broker", "enabled": true},
    {"username": "service-account-%s", "serviceAccountClientId": "%s", "enabled": true},
    {"username": "service-account-%s", "serviceAccountClientId": "%s", "enabled": true}
  ]
}
`, realmName, realmName, ttlLine, clientID, clientSecret, revokedClientID, revokedClientSecret, clientID, clientID, revokedClientID, revokedClientID)
}

func errorsJoin(errs ...error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return errors.Join(filtered...)
}
