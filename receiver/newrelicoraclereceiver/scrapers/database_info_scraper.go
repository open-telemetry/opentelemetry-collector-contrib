// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scrapers

import (
	"context"
	"database/sql"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/errors"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicoraclereceiver/queries"
)

// DatabaseInfoScraper collects Oracle database version and hosting environment info
type DatabaseInfoScraper struct {
	db           *sql.DB
	mb           *metadata.MetricsBuilder
	logger       *zap.Logger
	instanceName string
	config       metadata.MetricsBuilderConfig

	// Cache static info to avoid repeated DB queries
	cachedInfo      *DatabaseInfo
	cacheValidUntil time.Time
	cacheDuration   time.Duration
	cacheMutex      sync.RWMutex // Thread-safe cache access
}

// DatabaseInfo holds database and system environment details
type DatabaseInfo struct {
	// DB version info from Oracle instance
	Version     string
	VersionFull string
	Edition     string
	Compatible  string

	// System environment detected from host
	HostingProvider     string // aws, azure, gcp, oci, kubernetes, container, on-premises
	DeploymentPlatform  string // ec2, lambda, aks, docker, etc.
	CombinedEnvironment string // provider.platform format
	Architecture        string // amd64, arm64, 386
	OperatingSystem     string // linux, windows, darwin
}

// NewDatabaseInfoScraper creates a new database info scraper
func NewDatabaseInfoScraper(db *sql.DB, mb *metadata.MetricsBuilder, logger *zap.Logger, instanceName string, config metadata.MetricsBuilderConfig) *DatabaseInfoScraper {
	return &DatabaseInfoScraper{
		db:            db,
		mb:            mb,
		logger:        logger,
		instanceName:  instanceName,
		config:        config,
		cacheDuration: 1 * time.Hour, // Version info rarely changes
	}
}

// ScrapeDatabaseInfo collects Oracle version and config info
func (s *DatabaseInfoScraper) ScrapeDatabaseInfo(ctx context.Context) []error {
	var errs []error

	if !s.config.Metrics.NewrelicoracledbDatabaseInfo.Enabled {
		return errs
	}

	// Use cache to avoid repeated DB queries
	if err := s.ensureCacheValid(ctx); err != nil {
		errs = append(errs, err)
		return errs
	}

	// Read cached data safely
	s.cacheMutex.RLock()
	cachedInfo := s.cachedInfo
	s.cacheMutex.RUnlock()

	if cachedInfo == nil {
		s.logger.Warn("Database info cache is empty, skipping metrics recording")
		return errs
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	s.mb.RecordNewrelicoracledbDatabaseInfoDataPoint(
		now,
		int64(1),               // Info metric with value 1
		s.instanceName,         // db.instance.name
		cachedInfo.Version,     // db.version
		cachedInfo.VersionFull, // db.version.full
		cachedInfo.Edition,     // db.edition
		cachedInfo.Compatible,  // db.compatible
	)

	return errs
}

// ScrapeHostingInfo collects platform and environment info
func (s *DatabaseInfoScraper) ScrapeHostingInfo(ctx context.Context) []error {
	var errs []error

	if !s.config.Metrics.NewrelicoracledbHostingInfo.Enabled {
		return errs
	}

	// Use cache to avoid repeated system checks
	if err := s.ensureCacheValid(ctx); err != nil {
		errs = append(errs, err)
		return errs
	}

	// Read cached data safely
	s.cacheMutex.RLock()
	cachedInfo := s.cachedInfo
	s.cacheMutex.RUnlock()

	if cachedInfo == nil {
		s.logger.Warn("Database info cache is empty, skipping metrics recording")
		return errs
	}

	now := pcommon.NewTimestampFromTime(time.Now())
	s.mb.RecordNewrelicoracledbHostingInfoDataPoint(
		now,
		int64(1),       // Info metric with value 1
		s.instanceName, // db.instance.name
		extractCloudProviderForOTEL(cachedInfo.HostingProvider), // cloud.provider
		cachedInfo.DeploymentPlatform,                           // cloud.platform
		cachedInfo.CombinedEnvironment,                          // deployment.environment
		cachedInfo.Architecture,                                 // host.arch
		cachedInfo.OperatingSystem,                              // platform.name
	)

	return errs
}

// extractCloudProviderForOTEL extracts only actual cloud providers for OTEL cloud.provider attribute
func extractCloudProviderForOTEL(hostingProvider string) string {
	switch hostingProvider {
	case "aws", "azure", "gcp", "oci":
		return hostingProvider
	default:
		return "" // Return empty for non-cloud environments (kubernetes, container, on-premises)
	}
}

// ensureCacheValid checks if cache is valid and refreshes if needed with proper locking
func (s *DatabaseInfoScraper) ensureCacheValid(ctx context.Context) error {
	now := time.Now()

	// First check with read lock
	s.cacheMutex.RLock()
	if s.cachedInfo != nil && now.Before(s.cacheValidUntil) {
		s.logger.Debug("Using cached database info",
			zap.Time("cache_valid_until", s.cacheValidUntil),
			zap.Duration("time_remaining", s.cacheValidUntil.Sub(now)))
		s.cacheMutex.RUnlock()
		return nil
	}
	s.cacheMutex.RUnlock()

	// Cache is expired or empty, refresh with write lock
	s.cacheMutex.Lock()
	defer s.cacheMutex.Unlock()

	// Double-check pattern: another goroutine might have refreshed while we waited
	if s.cachedInfo != nil && now.Before(s.cacheValidUntil) {
		s.logger.Debug("Cache was refreshed by another goroutine")
		return nil
	}

	s.logger.Debug("Cache expired or empty, refreshing database info from database")
	return s.refreshCacheUnsafe(ctx)
}

// refreshCacheUnsafe performs the actual database query to refresh cached information
// Must be called with cacheMutex locked
func (s *DatabaseInfoScraper) refreshCacheUnsafe(ctx context.Context) error {
	s.logger.Debug("Executing database info query",
		zap.String("sql", errors.FormatQueryForLogging(queries.OptimizedDatabaseInfoSQL)))

	rows, err := s.db.QueryContext(ctx, queries.OptimizedDatabaseInfoSQL)
	if err != nil {
		s.logger.Error("Database info query failed", zap.Error(err))
		return err
	}
	defer rows.Close()

	return s.processDatabaseInfoRows(rows)
}

// processDatabaseInfoRows processes the query results and updates the cache
func (s *DatabaseInfoScraper) processDatabaseInfoRows(rows *sql.Rows) error {
	for rows.Next() {
		var instID, versionFull, hostName, databaseName, platformName sql.NullString

		if err := rows.Scan(&instID, &versionFull, &hostName, &databaseName, &platformName); err != nil {
			s.logger.Error("Failed to scan database info row", zap.Error(err))
			return err
		}

		// Extract and normalize database information from VERSION_FULL
		cleanVersion := extractVersionFromFull(versionFull.String)
		detectedEdition := detectEditionFromVersion(versionFull.String)

		// Combine environment detection with database-level hints for better accuracy
		hostingProvider, deploymentPlatform, combinedEnv := detectSystemEnvironmentWithDBHints(
			hostName.String,
			databaseName.String,
			platformName.String,
		)

		// Cache the information
		s.cachedInfo = &DatabaseInfo{
			Version:             cleanVersion,
			VersionFull:         versionFull.String,
			Edition:             detectedEdition,
			Compatible:          cleanVersion, // Use clean version as compatible
			HostingProvider:     hostingProvider,
			DeploymentPlatform:  deploymentPlatform,
			CombinedEnvironment: combinedEnv,
			Architecture:        runtime.GOARCH,
			OperatingSystem:     runtime.GOOS,
		}

		// Update cache expiry
		s.cacheValidUntil = time.Now().Add(s.cacheDuration)

		s.logger.Info("Database info cache refreshed successfully",
			zap.String("version", cleanVersion),
			zap.String("version_full", versionFull.String),
			zap.String("edition", detectedEdition),
			zap.String("hosting_provider", hostingProvider),
			zap.String("deployment_platform", deploymentPlatform),
			zap.String("combined_environment", combinedEnv),
			zap.String("db_hostname", hostName.String),
			zap.String("db_platform", platformName.String),
			zap.Time("cache_valid_until", s.cacheValidUntil))

		break // Only process first row
	}

	if err := rows.Err(); err != nil {
		s.logger.Error("Error iterating database info rows", zap.Error(err))
		return err
	}

	return nil
}

// Helper functions for data normalization following OpenTelemetry semantic conventions

// detectCloudProvider checks hostname for cloud provider patterns
func detectCloudProvider(hostname string) (provider, platform string) {
	hostname = strings.ToLower(hostname)

	// Check for container environments first (Docker containers have hex hostnames)
	if isContainerHostname(hostname) {
		return "", "container"
	}

	switch {
	case strings.Contains(hostname, "rds"):
		return "aws", "aws_rds"
	case strings.Contains(hostname, "ec2") || strings.Contains(hostname, "aws") || strings.Contains(hostname, "amazon"):
		return "aws", "aws_ec2"
	case strings.Contains(hostname, "azure-sql"):
		return "azure", "azure_sql_db"
	case strings.Contains(hostname, "azure") || strings.Contains(hostname, "microsoft"):
		return "azure", "azure_vm"
	case strings.Contains(hostname, "gcp") || strings.Contains(hostname, "google"):
		return "gcp", "gcp_compute_engine"
	case strings.Contains(hostname, "oci") || (strings.Contains(hostname, "oracle") && !strings.Contains(hostname, "database")):
		return "oci", "oci_compute"
	default:
		return "", ""
	}
}

// isContainerHostname checks if hostname looks like a Docker container ID
func isContainerHostname(hostname string) bool {
	// Docker containers typically have 12-character hex hostnames
	if len(hostname) == 12 {
		for _, char := range hostname {
			if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
				return false
			}
		}
		return true
	}
	return false
}

// extractArchitecture extracts and normalizes CPU architecture from platform name
func extractArchitecture(platformName string) string {
	platformName = strings.ToLower(platformName)
	switch {
	case strings.Contains(platformName, "x86_64") || strings.Contains(platformName, "amd64"):
		return "amd64"
	case strings.Contains(platformName, "aarch64") || strings.Contains(platformName, "arm64"):
		return "arm64"
	case strings.Contains(platformName, "i386") || strings.Contains(platformName, "i686"):
		return "386"
	default:
		return "unknown"
	}
}

// extractVersionNumber parses Oracle version from banner or version string
func extractVersionNumber(banner string) string {
	if banner == "" {
		return "unknown"
	}

	// Handle direct version format from GV$INSTANCE.VERSION (e.g., "23.9.0.25.07")
	if !strings.Contains(banner, "Oracle Database") {
		// This is likely a direct version number, clean it up
		versionParts := strings.Split(strings.TrimSpace(banner), ".")
		if len(versionParts) >= 2 {
			// Return major.minor format for consistency (e.g., "23.9")
			return strings.Join(versionParts[:2], ".")
		}
		return banner
	}

	// Handle full banner format from V$VERSION.BANNER
	// Examples:
	// "Oracle Database 23ai Free Release 23.0.0.0.0 - Develop, Learn, and Run for Free"
	// "Oracle Database 19c Enterprise Edition Release 19.0.0.0.0 - Production"
	// "Oracle Database 21c Express Edition Release 21.0.0.0.0 - Production"
	// "Oracle Database 11g Enterprise Edition Release 11.2.0.4.0 - 64bit Production"

	// Extract version from "Release X.Y.Z.W.V" pattern
	if strings.Contains(banner, "Release ") {
		parts := strings.Split(banner, "Release ")
		if len(parts) > 1 {
			version := strings.Fields(parts[1])[0]
			// Return major.minor format (e.g., "23.0" from "23.0.0.0.0")
			versionParts := strings.Split(version, ".")
			if len(versionParts) >= 2 {
				return strings.Join(versionParts[:2], ".")
			}
			return version
		}
	}

	// Extract version from "Oracle Database XXai" or "Oracle Database XXc" pattern
	if strings.Contains(banner, "Oracle Database ") {
		parts := strings.Split(banner, "Oracle Database ")
		if len(parts) > 1 {
			versionPart := strings.Fields(parts[1])[0]
			// Handle formats like "23ai", "19c", "21c", "11g"
			if strings.HasSuffix(versionPart, "ai") {
				return strings.TrimSuffix(versionPart, "ai")
			}
			if strings.HasSuffix(versionPart, "c") || strings.HasSuffix(versionPart, "g") {
				return strings.TrimSuffix(strings.TrimSuffix(versionPart, "c"), "g")
			}
			return versionPart
		}
	}

	return "unknown"
}

func determineDeploymentEnvironment(cloudProvider, cloudPlatform string) string {
	if cloudProvider != "" {
		return "cloud"
	}
	if cloudPlatform == "container" {
		return "container"
	}
	return "on-premises"
}

// normalizePlatformName converts platform string to standard format
func normalizePlatformName(platform string) string {
	platform = strings.ToLower(platform)
	switch {
	case strings.Contains(platform, "linux"):
		return "linux"
	case strings.Contains(platform, "windows"):
		return "windows"
	case strings.Contains(platform, "solaris"):
		return "solaris"
	case strings.Contains(platform, "aix"):
		return "aix"
	case strings.Contains(platform, "hp-ux"):
		return "hpux"
	default:
		return "unknown"
	}
}

// detectSystemEnvironmentWithDBHints combines environment variables with database-level hints
// for more accurate cloud provider detection. Uses cached database query results.
func detectSystemEnvironmentWithDBHints(hostName, databaseName, platformName string) (hostingProvider, deploymentPlatform, combinedEnvironment string) {
	// First, try environment variable detection (highest priority)
	envProvider, envPlatform, envCombined := detectSystemEnvironment()
	if envProvider != "on-premises" {
		// Environment variables provided clear cloud provider info
		return envProvider, envPlatform, envCombined
	}

	// If environment variables don't reveal cloud provider, use database hints
	dbProvider, dbPlatform := detectCloudFromDatabaseHints(hostName, databaseName, platformName)
	if dbProvider != "" {
		// Database hints indicate cloud provider
		return dbProvider, dbPlatform, dbProvider + "." + dbPlatform
	}

	// Fallback to original environment detection (includes DMI)
	return envProvider, envPlatform, envCombined
}

// detectCloudFromDatabaseHints analyzes database-level information for cloud provider hints
func detectCloudFromDatabaseHints(hostName, databaseName, platformName string) (provider, platform string) {
	hostName = strings.ToLower(hostName)
	databaseName = strings.ToLower(databaseName)
	platformName = strings.ToLower(platformName)

	// AWS RDS detection
	if strings.Contains(hostName, "rds.amazonaws.com") ||
		strings.Contains(hostName, "rds-") ||
		strings.Contains(databaseName, "rds") {
		return "aws", "rds"
	}

	// AWS EC2 detection
	if strings.Contains(hostName, "ec2") ||
		strings.Contains(hostName, ".amazonaws.com") ||
		strings.Contains(hostName, "ip-") { // EC2 internal hostnames often start with ip-
		return "aws", "ec2"
	}

	// Azure SQL Database detection
	if strings.Contains(hostName, "database.windows.net") ||
		strings.Contains(hostName, "database.azure.com") {
		return "azure", "azure_sql_db"
	}

	// Azure VM detection
	if strings.Contains(hostName, ".cloudapp.azure.com") ||
		strings.Contains(hostName, "azure") ||
		strings.Contains(platformName, "microsoft") {
		return "azure", "azure_vm"
	}

	// Google Cloud SQL detection
	if strings.Contains(hostName, "googleusercontent.com") ||
		strings.Contains(hostName, "gcp-") {
		return "gcp", "cloud_sql"
	}

	// Google Compute Engine detection
	if strings.Contains(hostName, "c.googlecompute.com") ||
		strings.Contains(hostName, "gce-") ||
		strings.Contains(platformName, "google") {
		return "gcp", "compute_engine"
	}

	// Oracle Cloud Infrastructure detection
	if strings.Contains(hostName, "oraclecloud.com") ||
		strings.Contains(hostName, "oci-") ||
		(strings.Contains(platformName, "oracle") && !strings.Contains(platformName, "database")) {
		return "oci", "compute"
	}

	return "", ""
}

// detectSystemEnvironment finds hosting environment using system info
func detectSystemEnvironment() (hostingProvider, deploymentPlatform, combinedEnvironment string) {
	// Check for AWS environment variables
	if checkEnvVar("AWS_REGION") || checkEnvVar("AWS_EXECUTION_ENV") || checkEnvVar("EC2_INSTANCE_ID") {
		if checkEnvVar("AWS_LAMBDA_FUNCTION_NAME") {
			return "aws", "lambda", "aws.lambda"
		}
		if strings.Contains(os.Getenv("AWS_EXECUTION_ENV"), "Fargate") {
			return "aws", "fargate", "aws.fargate"
		}
		return "aws", "ec2", "aws.ec2"
	}

	// Check for Azure environment variables (App Service, Functions, etc.)
	if checkEnvVar("AZURE_CLIENT_ID") || checkEnvVar("WEBSITE_INSTANCE_ID") || checkEnvVar("AZURE_SUBSCRIPTION_ID") {
		if checkEnvVar("WEBSITE_INSTANCE_ID") {
			return "azure", "app-service", "azure.app-service"
		}
		if checkEnvVar("AZURE_FUNCTIONS_ENVIRONMENT") {
			return "azure", "functions", "azure.functions"
		}
		return "azure", "vm", "azure.vm"
	}

	// Check for additional Azure environment variables commonly set on Azure VMs
	if checkEnvVar("AZURE_TENANT_ID") || checkEnvVar("MSI_ENDPOINT") || checkEnvVar("AZURE_RESOURCE_GROUP") {
		return "azure", "azure_vm", "azure.vm"
	}

	// Check for Google Cloud environment variables
	if checkEnvVar("GOOGLE_CLOUD_PROJECT") || checkEnvVar("GCP_PROJECT") || checkEnvVar("FUNCTION_NAME") {
		if checkEnvVar("FUNCTION_NAME") {
			return "gcp", "cloud-functions", "gcp.cloud-functions"
		}
		if checkEnvVar("K_SERVICE") {
			return "gcp", "cloud-run", "gcp.cloud-run"
		}
		return "gcp", "compute-engine", "gcp.compute-engine"
	}

	// Check for Oracle Cloud Infrastructure
	if checkEnvVar("OCI_RESOURCE_PRINCIPAL_VERSION") || checkEnvVar("OCI_REGION") {
		return "oci", "compute", "oci.compute"
	}

	// Check for Kubernetes
	if checkEnvVar("KUBERNETES_SERVICE_HOST") || fileExists("/var/run/secrets/kubernetes.io/serviceaccount") {
		return "kubernetes", "generic", "kubernetes.generic"
	}

	// Check for container environment
	if fileExists("/.dockerenv") || containsAny(readFile("/proc/1/cgroup"), []string{"/docker/", "/kubepods/", "/containerd/"}) {
		return "container", "docker", "container.docker"
	}

	// Check for cloud provider hints in DMI/BIOS information (Linux-specific, read-only)
	if runtime.GOOS == "linux" {
		if provider := detectCloudFromDMI(); provider != "" {
			switch provider {
			case "amazon":
				return "aws", "ec2", "aws.ec2"
			case "microsoft":
				return "azure", "azure_vm", "azure.vm"
			case "google":
				return "gcp", "compute-engine", "gcp.compute-engine"
			case "oracle":
				return "oci", "compute", "oci.compute"
			}
		}
	}

	// Default to on-premises
	return "on-premises", "bare-metal", "on-premises.bare-metal"
}

// Helper functions for environment detection
func checkEnvVar(name string) bool {
	return os.Getenv(name) != ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func containsAny(text string, substrings []string) bool {
	for _, substring := range substrings {
		if strings.Contains(text, substring) {
			return true
		}
	}
	return false
}

// detectCloudFromDMI reads DMI/BIOS information to detect cloud providers (Linux only)
// This is safe as it only reads system information files
func detectCloudFromDMI() string {
	// Read system vendor information
	vendor := strings.ToLower(readFile("/sys/class/dmi/id/sys_vendor"))
	product := strings.ToLower(readFile("/sys/class/dmi/id/product_name"))
	bios := strings.ToLower(readFile("/sys/class/dmi/id/bios_vendor"))

	// Amazon/AWS detection
	if strings.Contains(vendor, "amazon") || strings.Contains(product, "amazon") ||
		strings.Contains(bios, "amazon") || strings.Contains(product, "ec2") {
		return "amazon"
	}

	// Microsoft Azure detection
	if strings.Contains(vendor, "microsoft") || strings.Contains(product, "virtual machine") {
		return "microsoft"
	}

	// Google Cloud detection
	if strings.Contains(vendor, "google") || strings.Contains(product, "google") ||
		strings.Contains(product, "google compute engine") {
		return "google"
	}

	// Oracle Cloud detection
	if strings.Contains(vendor, "oracle") && strings.Contains(product, "oracle") {
		return "oracle"
	}

	return ""
}

// extractVersionFromFull extracts clean version number from GV$INSTANCE.VERSION
// Examples: "23.0.0.0.0" → "23.0", "19.3.0.0.0" → "19.3", "11.2.0.4.0" → "11.2"
func extractVersionFromFull(versionFull string) string {
	if versionFull == "" {
		return "unknown"
	}

	// Split by dots and take first two parts for major.minor format
	versionParts := strings.Split(strings.TrimSpace(versionFull), ".")
	if len(versionParts) >= 2 {
		return strings.Join(versionParts[:2], ".")
	}

	// If only one part, return as-is
	if len(versionParts) == 1 {
		return versionParts[0]
	}

	return versionFull
}

// detectEditionFromVersion detects Oracle edition from version pattern
// This is a best-effort detection based on common version patterns
func detectEditionFromVersion(versionFull string) string {
	if versionFull == "" {
		return "unknown"
	}

	// Extract major version number
	majorVersion := strings.Split(versionFull, ".")[0]

	// Oracle 23ai is typically Free edition in container environments
	if majorVersion == "23" {
		return "free" // Oracle 23ai Free is the most common 23.x version
	}

	// For other versions, we can't reliably determine edition from version alone
	// Default to standard as it's the most common
	return "standard"
}
