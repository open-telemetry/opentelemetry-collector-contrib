// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package discovery // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver/internal/discovery"

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/alexbrainman/sspi"
	"github.com/alexbrainman/sspi/ntlm"
	"github.com/go-ldap/ldap/v3"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"

	stanza "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza/operator/input/windows"
)

// sspINegotiator implements ldap.NTLMNegotiator using current user's SSPI credentials.
// This mirrors C++ ADsOpenObject(..., ADS_SECURE_AUTHENTICATION) with null username/password.
type sspINegotiator struct {
	creds *sspi.Credentials   // current user creds acquired from SSPI
	ctx   *ntlm.ClientContext // NTLM client context for challenge/response
}

var _ ldap.NTLMNegotiator = (*sspINegotiator)(nil)

// Acquire current logged-on user credentials no username/password needed
func (n *sspINegotiator) Negotiate(_, _ string) ([]byte, error) {
	creds, err := ntlm.AcquireCurrentUserCredentials()
	if err != nil {
		return nil, fmt.Errorf("failed to acquire current user SSPI credentials: %w", err)
	}
	n.creds = creds

	// Create NTLM client context and generate Type 1 (Negotiate) message
	ctx, negotiateMsg, err := ntlm.NewClientContext(creds)
	if err != nil {
		_ = creds.Release()
		return nil, fmt.Errorf("failed to create NTLM client context: %w", err)
	}
	n.ctx = ctx
	return negotiateMsg, nil
}

func (n *sspINegotiator) ChallengeResponse(challenge []byte, _, _ string) ([]byte, error) {
	// Process Type 2 (Challenge) from server, produce Type 3 (Authenticate) message
	authenticateMsg, err := n.ctx.Update(challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to process NTLM challenge: %w", err)
	}
	return authenticateMsg, nil
}

func (n *sspINegotiator) Release() {
	if n.ctx != nil {
		_ = n.ctx.Release()
	}
	if n.creds != nil {
		_ = n.creds.Release()
	}
}

// getLDAPDomainPath discovers the root domain path of the Active Directory service.
// It first tries querying the LDAP Root DSE, then falls back to the Windows API.
// Returns a path like "LDAP://DC=example,DC=com".
func getLDAPDomainPath(logger *zap.Logger) (string, error) {
	currentJoinedDomain, currentDomainError := getCurrentMachineJoinedDomain()
	if currentDomainError != nil {
		return "", fmt.Errorf("failed to get current machine joined domain: %w", currentDomainError)
	}

	logger.Info("current machine joined domain is ", zap.String("currentJoinedDomain", currentJoinedDomain))

	// Primary: query Root DSE with the current joined domain as the LDAP server
	path, err := getRootLDAPDomainPath(currentJoinedDomain)
	if err == nil {
		return path, nil
	}
	logger.Error("return current join domain path, error while getting rootDomainPath using ldap ", zap.Error(err))
	// Fallback: current Joined Domain
	return currentJoinedDomain, nil
}

// getRootLDAPDomainPath connects to the current machine joined DC and reads
// the defaultNamingContext attribute.
func getRootLDAPDomainPath(domain string) (string, error) {
	conn, err := ldap.DialURL("ldap://"+domain, ldap.DialWithDialer(&net.Dialer{Timeout: 20 * time.Second}))
	if err != nil {
		return "", fmt.Errorf("failed to connect to LDAP Root DSE: %w", err)
	}
	defer conn.Close()

	req := ldap.NewSearchRequest(
		"",                   // Base DN: empty string = Root DSE
		ldap.ScopeBaseObject, // Only the root entry itself
		ldap.NeverDerefAliases,
		1,     // Size limit of 1 since we only expect one Root DSE entry
		10,    // 10s time limit to avoid hanging if something is wrong with the server
		false, // attrs only = false
		"(objectClass=*)",
		[]string{"defaultNamingContext"},
		nil,
	)

	res, err := conn.Search(req)
	if err != nil {
		return "", fmt.Errorf("LDAP Root DSE search failed: %w", err)
	}

	if len(res.Entries) == 0 {
		return "", errors.New("LDAP Root DSE returned no entries")
	}

	namingContext := res.Entries[0].GetAttributeValue("defaultNamingContext")
	if namingContext == "" {
		return "", errors.New("defaultNamingContext attribute is empty")
	}

	// namingContext is already in DN format, e.g. "DC=example,DC=com"
	return dnToHostname(namingContext), nil
}

// getLDAPDomainPathFromWindowsAPI uses GetComputerNameEx to get the DNS domain
// name and converts it to an LDAP path.
func getCurrentMachineJoinedDomain() (string, error) {
	// First call to get required buffer size
	var size uint32
	err := windows.GetComputerNameEx(windows.ComputerNameDnsDomain, nil, &size)
	if err != nil && !errors.Is(err, windows.ERROR_MORE_DATA) {
		return "", fmt.Errorf("GetComputerNameEx (size query) failed: %w", err)
	}

	if size == 0 {
		return "", errors.New("computer is not joined to a domain")
	}

	buf := make([]uint16, size)
	err = windows.GetComputerNameEx(windows.ComputerNameDnsDomain, &buf[0], &size)
	if err != nil {
		return "", fmt.Errorf("GetComputerNameEx failed: %w", err)
	}

	// Decode UTF-16 to string, trimming null terminator
	dnsDomain := windows.UTF16ToString(buf[:size])
	if dnsDomain == "" {
		return "", errors.New("computer is not joined to a domain (empty DNS domain)")
	}

	return dnsDomain, nil
}

func discoverDomainControllersForJoinedDomain(logger *zap.Logger) (string, []string, error) {
	domain, err := getLDAPDomainPath(logger)
	if err != nil {
		return "", nil, err
	}
	logger.Info("root domain: " + domain)

	domainControllers, err := getDomainControllersForDomain(domain)
	if err != nil {
		return domain, nil, err
	}
	return domain, domainControllers, nil
}

func getDomainControllersForDomain(domain string) ([]string, error) {
	conn, err := ldap.DialURL("ldap://"+domain, ldap.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP server %s: %w", domain, err)
	}
	defer conn.Close()

	domainDN := domainToDN(domain)

	negotiator := &sspINegotiator{}
	defer negotiator.Release()

	_, err = conn.NTLMChallengeBind(&ldap.NTLMBindRequest{
		Negotiator:         negotiator,
		AllowEmptyPassword: true,
	})
	if err != nil {
		return nil, fmt.Errorf("SSPI NTLM bind failed: %w", err)
	}

	searchRequest := ldap.NewSearchRequest(
		domainDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1000,  // Size limit, keeping 1000 for safety, though we expect far fewer DCs
		30,    // Time limit of 30s to avoid hanging if something is wrong with the server
		false, // Types only
		"(&(objectClass=computer)(primaryGroupID=516))",      // LDAP filter for DCs
		[]string{"dNSHostName", "name", "distinguishedName"}, // Attributes to retrieve
		nil,
	)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed for domain %s: %w", domain, err)
	}

	var domainControllers []string
	for _, entry := range sr.Entries {
		dc := entry.GetAttributeValue("dNSHostName")
		if dc != "" {
			domainControllers = append(domainControllers, dc)
		}
	}

	return domainControllers, nil
}

// domainToDN converts a domain name to an LDAP Distinguished Name.
// Example: "example.com" -> "DC=example,DC=com"
func domainToDN(domain string) string {
	parts := strings.Split(domain, ".")
	return "DC=" + strings.Join(parts, ",DC=")
}

func dnToHostname(dn string) string {
	parts := strings.Split(dn, ",")
	labels := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(strings.ToUpper(p), "DC=") {
			labels = append(labels, p[3:])
		}
	}
	return strings.Join(labels, ".")
}

// GetJoinedDomainControllersRemoteConfig is a variable holding the function that discovers domain controllers
// for the machine's joined domain and returns a RemoteConfig slice for each discovered controller.
// It is a variable to allow mocking in tests.
var GetJoinedDomainControllersRemoteConfig = func(logger *zap.Logger, username, password string) ([]stanza.RemoteConfig, error) {
	var domainControllerConfigs []stanza.RemoteConfig
	domain, domainControllers, err := discoverDomainControllersForJoinedDomain(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to discover domain controllers: %w", err)
	}
	if len(domainControllers) == 0 {
		return nil, errors.New("no domain controllers found during discovery")
	}
	for _, dc := range domainControllers {
		config := stanza.RemoteConfig{Server: dc, Username: username, Password: password, Domain: domain}
		domainControllerConfigs = append(domainControllerConfigs, config)
	}
	logger.Info("Discovered domain controllers", zap.Int("count", len(domainControllers)))
	return domainControllerConfigs, nil
}
