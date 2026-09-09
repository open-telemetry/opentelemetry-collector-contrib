// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package supervisor

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/open-telemetry/opamp-go/client/types"
	"github.com/open-telemetry/opamp-go/protobufs"
	"go.uber.org/zap"
)

// packageManager manages the persistent state of downloadable packages.
// Currently only allows for a single top-level package containing the agent.
type packageManager struct {
	logger *zap.Logger
}

var _ types.PackagesStateProvider = &packageManager{}

func (p *packageManager) AllPackagesHash() ([]byte, error) {
	p.logger.Debug("AllPackagesHash not yet implemented")
	return nil, fmt.Errorf("AllPackagesHash: %w", errors.ErrUnsupported)
}

func (p *packageManager) SetAllPackagesHash(_ []byte) error {
	p.logger.Debug("SetAllPackagesHash not yet implemented")
	return fmt.Errorf("SetAllPackagesHash: %w", errors.ErrUnsupported)
}

func (p *packageManager) Packages() ([]string, error) {
	p.logger.Debug("Packages not yet implemented")
	return nil, fmt.Errorf("Packages: %w", errors.ErrUnsupported)
}

func (p *packageManager) PackageState(packageName string) (types.PackageState, error) {
	p.logger.Debug("PackageState not yet implemented", zap.String("package", packageName))
	return types.PackageState{Exists: false}, fmt.Errorf("PackageState: %w", errors.ErrUnsupported)
}

func (p *packageManager) SetPackageState(packageName string, _ types.PackageState) error {
	p.logger.Debug("SetPackageState not yet implemented", zap.String("package", packageName))
	return fmt.Errorf("SetPackageState: %w", errors.ErrUnsupported)
}

func (p *packageManager) CreatePackage(packageName string, _ protobufs.PackageType) error {
	p.logger.Debug("CreatePackage not yet implemented", zap.String("package", packageName))
	return fmt.Errorf("CreatePackage: %w", errors.ErrUnsupported)
}

func (p *packageManager) FileContentHash(packageName string) ([]byte, error) {
	p.logger.Debug("FileContentHash not yet implemented", zap.String("package", packageName))
	return nil, fmt.Errorf("FileContentHash: %w", errors.ErrUnsupported)
}

func (p *packageManager) UpdateContent(_ context.Context, packageName string, _ io.Reader, _, _ []byte) error {
	p.logger.Debug("UpdateContent not yet implemented", zap.String("package", packageName))
	return fmt.Errorf("UpdateContent: %w", errors.ErrUnsupported)
}

func (p *packageManager) DeletePackage(packageName string) error {
	p.logger.Debug("DeletePackage not yet implemented", zap.String("package", packageName))
	return fmt.Errorf("DeletePackage: %w", errors.ErrUnsupported)
}

func (p *packageManager) LastReportedStatuses() (*protobufs.PackageStatuses, error) {
	p.logger.Debug("LastReportedStatuses not yet implemented")
	return nil, fmt.Errorf("LastReportedStatuses: %w", errors.ErrUnsupported)
}

func (p *packageManager) SetLastReportedStatuses(_ *protobufs.PackageStatuses) error {
	p.logger.Debug("SetLastReportedStatuses not yet implemented")
	return fmt.Errorf("SetLastReportedStatuses: %w", errors.ErrUnsupported)
}
