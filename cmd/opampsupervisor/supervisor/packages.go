// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/addon"
)

const (
	// packageStateFile holds the Supervisor's record of which packages it has.
	packageStateFile = "state.json"
	// packageStatusesFile holds the last PackageStatuses reported to the server,
	// so that a restarted Supervisor reports what it has rather than nothing.
	packageStatusesFile = "last_reported_statuses.bin"
	// packageContentFile is the package file exactly as downloaded. It is kept so
	// that FileContentHash can answer without the server re-sending the file.
	packageContentFile = "content"
	// packageFilesDir is where an addon package's contents are extracted.
	packageFilesDir = "files"
)

// packageRecord is the Supervisor's persisted record of one package.
type packageRecord struct {
	Type    protobufs.PackageType `json:"type"`
	Hash    []byte                `json:"hash,omitempty"`
	Version string                `json:"version,omitempty"`
}

// packageState is the on-disk state of all packages.
type packageState struct {
	AllPackagesHash []byte                    `json:"all_packages_hash,omitempty"`
	Packages        map[string]*packageRecord `json:"packages"`
}

// packageManager manages the persistent state of downloadable packages.
//
// Top-level packages carry the Collector executable and are installed by the
// agent update path. Addon packages are named "<type>:<name>" and are installed
// and run by the factory registered for their type.
type packageManager struct {
	logger  *zap.Logger
	rootDir string
	addons  *addon.Manager

	// mu guards both the in-memory state and the files backing it. The syncer
	// may call these methods concurrently when several packages are offered.
	mu    sync.Mutex
	state packageState
}

var _ types.PackagesStateProvider = &packageManager{}

// newPackageManager returns a packageManager storing packages under rootDir,
// loading any state left by a previous run.
func newPackageManager(logger *zap.Logger, rootDir string, addons *addon.Manager) (*packageManager, error) {
	if err := os.MkdirAll(rootDir, 0o700); err != nil {
		return nil, fmt.Errorf("create package directory: %w", err)
	}

	p := &packageManager{
		logger:  logger,
		rootDir: rootDir,
		addons:  addons,
		state:   packageState{Packages: map[string]*packageRecord{}},
	}

	if err := p.load(); err != nil {
		return nil, err
	}

	return p, nil
}

// load reads persisted state. Missing state is not an error: it means this is
// the first run.
func (p *packageManager) load() error {
	data, err := os.ReadFile(p.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read package state: %w", err)
	}

	var state packageState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("parse package state: %w", err)
	}
	if state.Packages == nil {
		state.Packages = map[string]*packageRecord{}
	}
	p.state = state

	return nil
}

// persist writes state atomically so that a crash mid-write cannot leave the
// Supervisor unable to parse its own record of what is installed.
func (p *packageManager) persist() error {
	data, err := json.Marshal(p.state)
	if err != nil {
		return fmt.Errorf("marshal package state: %w", err)
	}
	return writeFileAtomic(p.statePath(), data, 0o600)
}

func (p *packageManager) statePath() string {
	return filepath.Join(p.rootDir, packageStateFile)
}

// packageDir returns the directory holding one package. Package names are
// server-chosen, so the name is escaped rather than used as a path. Every
// character other than letters, digits and "-_.~" is escaped, not only path
// separators: addon names always contain ":", which Windows forbids in file
// names, and which the JVM treats as a class path separator, so a jar under a
// directory named "jmx:tomcat" cannot be run with "java -jar".
func (p *packageManager) packageDir(packageName string) string {
	return filepath.Join(p.rootDir, url.QueryEscape(packageName))
}

func (p *packageManager) contentPath(packageName string) string {
	return filepath.Join(p.packageDir(packageName), packageContentFile)
}

// AddonDir returns the directory an addon package's files are extracted to.
func (p *packageManager) AddonDir(packageName string) string {
	return filepath.Join(p.packageDir(packageName), packageFilesDir)
}

func (p *packageManager) AllPackagesHash() ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state.AllPackagesHash, nil
}

func (p *packageManager) SetAllPackagesHash(hash []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.state.AllPackagesHash = hash
	return p.persist()
}

func (p *packageManager) Packages() ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	names := make([]string, 0, len(p.state.Packages))
	for name := range p.state.Packages {
		names = append(names, name)
	}
	return names, nil
}

func (p *packageManager) PackageState(packageName string) (types.PackageState, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	record, ok := p.state.Packages[packageName]
	if !ok {
		return types.PackageState{Exists: false}, nil
	}

	return types.PackageState{
		Exists:  true,
		Type:    record.Type,
		Hash:    record.Hash,
		Version: record.Version,
	}, nil
}

func (p *packageManager) SetPackageState(packageName string, state types.PackageState) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	record, ok := p.state.Packages[packageName]
	if !ok {
		return fmt.Errorf("package %q does not exist", packageName)
	}
	// The syncer deletes and recreates a package whose type changes, so a type
	// change here means the caller and the stored record have diverged.
	if record.Type != state.Type {
		return fmt.Errorf("package %q is of type %v, cannot set state with type %v",
			packageName, record.Type, state.Type)
	}

	record.Hash = state.Hash
	record.Version = state.Version

	return p.persist()
}

func (p *packageManager) CreatePackage(packageName string, typ protobufs.PackageType) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, ok := p.state.Packages[packageName]; ok {
		return fmt.Errorf("package %q already exists", packageName)
	}

	// The syncer calls CreatePackage before downloading, and reports an error
	// as INSTALL_FAILED, so an addon of an unsupported type is rejected without
	// fetching it.
	if typ == protobufs.PackageType_PackageType_Addon {
		if err := p.addons.Supports(packageName); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(p.packageDir(packageName), 0o700); err != nil {
		return fmt.Errorf("create directory for package %q: %w", packageName, err)
	}

	p.state.Packages[packageName] = &packageRecord{Type: typ}

	return p.persist()
}

// FileContentHash returns the hash of the package file the Supervisor has, so
// the syncer can skip downloading a file that has not changed. It is computed
// from the stored file rather than remembered, so it cannot drift from what is
// actually on disk.
func (p *packageManager) FileContentHash(packageName string) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	f, err := os.Open(p.contentPath(packageName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open content for package %q: %w", packageName, err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("hash content for package %q: %w", packageName, err)
	}

	return hasher.Sum(nil), nil
}

// UpdateContent stores the downloaded package file and, for an addon, extracts
// it and registers it. The content hash is verified before anything is
// extracted or run.
func (p *packageManager) UpdateContent(
	ctx context.Context,
	packageName, downloadURL string,
	data io.Reader,
	contentHash, signature []byte,
) error {
	p.mu.Lock()
	record, ok := p.state.Packages[packageName]
	p.mu.Unlock()
	if !ok {
		return fmt.Errorf("package %q does not exist", packageName)
	}

	content, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("read content for package %q: %w", packageName, err)
	}

	// Verify before writing anything executable to disk. A mismatch means the
	// bytes are not what the server described, so the package is rejected
	// outright rather than installed and reported as working.
	if len(contentHash) > 0 {
		sum := sha256.Sum256(content)
		if subtle.ConstantTimeCompare(sum[:], contentHash) != 1 {
			return fmt.Errorf("package %q content hash mismatch: got %s, offered %s",
				packageName, hex.EncodeToString(sum[:]), hex.EncodeToString(contentHash))
		}
	}

	if err := writeFileAtomic(p.contentPath(packageName), content, 0o600); err != nil {
		return fmt.Errorf("store content for package %q: %w", packageName, err)
	}

	if record.Type != protobufs.PackageType_PackageType_Addon {
		// Top-level packages are installed by the agent update path, which reads
		// the stored content. Nothing further to do here.
		return nil
	}

	return p.installAddon(ctx, packageName, content, downloadURL, signature)
}

// installAddon installs an addon package into a fresh directory using its
// type's factory, then swaps it into place. A failing install leaves the
// previous version installed and running.
func (p *packageManager) installAddon(ctx context.Context, packageName string, content []byte, downloadURL string, signature []byte) error {
	finalDir := p.AddonDir(packageName)
	stagingDir := finalDir + ".new"

	if err := os.RemoveAll(stagingDir); err != nil {
		return fmt.Errorf("clear staging directory for addon %q: %w", packageName, err)
	}
	if err := os.MkdirAll(stagingDir, 0o700); err != nil {
		return fmt.Errorf("create staging directory for addon %q: %w", packageName, err)
	}

	if err := p.addons.Install(ctx, packageName, stagingDir, content, downloadURL, signature); err != nil {
		_ = os.RemoveAll(stagingDir)
		return fmt.Errorf("install addon %q: %w", packageName, err)
	}

	// Shut the running addon down before replacing its files. On Windows a
	// running executable cannot be replaced, and on any platform a process
	// reading files that are being swapped is a race.
	if err := p.addons.Unload(ctx, packageName); err != nil {
		p.logger.Warn("Could not shut down addon before replacing its files",
			zap.String("addon", packageName), zap.Error(err))
	}

	if err := os.RemoveAll(finalDir); err != nil {
		return fmt.Errorf("remove previous addon %q: %w", packageName, err)
	}
	if err := os.Rename(stagingDir, finalDir); err != nil {
		return fmt.Errorf("install addon %q: %w", packageName, err)
	}

	if err := p.addons.Load(ctx, packageName, finalDir); err != nil {
		return err
	}

	p.logger.Info("Addon package installed", zap.String("addon", packageName), zap.String("dir", finalDir))

	return nil
}

func (p *packageManager) DeletePackage(packageName string) error {
	// Stop the process before removing files, and do it outside the lock since
	// stopping waits on the process to exit.
	if err := p.addons.Unload(context.Background(), packageName); err != nil {
		p.logger.Warn("Could not shut down addon before deleting it",
			zap.String("addon", packageName), zap.Error(err))
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if err := os.RemoveAll(p.packageDir(packageName)); err != nil {
		return fmt.Errorf("remove package %q: %w", packageName, err)
	}

	delete(p.state.Packages, packageName)

	p.logger.Info("Package deleted", zap.String("package", packageName))

	return p.persist()
}

func (p *packageManager) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := os.ReadFile(filepath.Join(p.rootDir, packageStatusesFile))
	if errors.Is(err, os.ErrNotExist) {
		return &protobufs.PackageStatuses{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read last reported statuses: %w", err)
	}

	statuses := &protobufs.PackageStatuses{}
	if err := proto.Unmarshal(data, statuses); err != nil {
		return nil, fmt.Errorf("parse last reported statuses: %w", err)
	}

	return statuses, nil
}

func (p *packageManager) SetLastReportedStatuses(statuses *protobufs.PackageStatuses) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	data, err := proto.Marshal(statuses)
	if err != nil {
		return fmt.Errorf("marshal last reported statuses: %w", err)
	}

	return writeFileAtomic(filepath.Join(p.rootDir, packageStatusesFile), data, 0o600)
}

// loadInstalledAddons loads every addon already on disk. It runs at startup so
// that a restarted Supervisor adopts the addons it installed previously without
// waiting for the server to re-offer them. Nothing is started here; the
// startup reconciliation does that once the persisted remote config is known.
func (p *packageManager) loadInstalledAddons(ctx context.Context) error {
	p.mu.Lock()
	names := make([]string, 0, len(p.state.Packages))
	for name, record := range p.state.Packages {
		if record.Type == protobufs.PackageType_PackageType_Addon {
			names = append(names, name)
		}
	}
	p.mu.Unlock()

	var errs []error
	for _, name := range names {
		dir := p.AddonDir(name)
		if _, err := os.Stat(dir); err != nil {
			errs = append(errs, fmt.Errorf("addon %q is recorded but not present at %s: %w", name, dir, err))
			continue
		}
		if err := p.addons.Load(ctx, name, dir); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// writeFileAtomic writes data to path by writing a sibling temporary file and
// renaming it, so readers never observe a partially written file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		tmp.Close()
		_ = os.Remove(tmpName)
	}()

	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}
