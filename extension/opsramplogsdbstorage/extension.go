//go:build !freebsd

package opsramplogsdbstorage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"go.opentelemetry.io/collector/extension"

	"github.com/syndtr/goleveldb/leveldb"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension/experimental/storage"
	"go.uber.org/zap"
)

type databaseStorage struct {
	dbPath string
	logger *zap.Logger
	db     *leveldb.DB
}

// Ensure this storage extension implements the appropriate interface
var _ storage.Extension = (*databaseStorage)(nil)

func newDBStorage(logger *zap.Logger, config *Config) (extension.Extension, error) {
	return &databaseStorage{
		dbPath: config.DBPath,
		logger: logger,
	}, nil
}

// Start opens a connection to the database
func (ds *databaseStorage) Start(context.Context, component.Host) error {
	db, err := leveldb.OpenFile(ds.dbPath, nil)
	if err != nil {
		return err
	}

	ds.db = db
	return nil
}

// Shutdown closes the connection to the database
func (ds *databaseStorage) Shutdown(context.Context) (shutdownErr error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			shutdownErr = fmt.Errorf("handiling panic error: %v", panicErr)
			// remove ds.dbpath
			if err := os.RemoveAll(ds.dbPath); err != nil {
				shutdownErr = fmt.Errorf("handiling panic error: %v and Directory %v removing error: %v", panicErr, ds.dbPath, err)
			}
		}
	}()
	// ds.db nil return error
	if ds.db == nil {
		if err := os.RemoveAll(ds.dbPath); err != nil {
			shutdownErr = fmt.Errorf("directory %v removing error: %v", ds.dbPath, err)
		}
		return shutdownErr
	}
	shutdownErr = ds.db.Close()
	if errors.Is(shutdownErr, leveldb.ErrClosed) {
		return nil
	}

	return shutdownErr
}

// GetClient returns a storage client for an individual component
func (ds *databaseStorage) GetClient(_ context.Context, kind component.Kind, ent component.ID, name string) (storage.Client, error) {
	var fullName string
	if name == "" {
		fullName = fmt.Sprintf("%s_%s_%s", kindString(kind), ent.Type(), ent.Name())
	} else {
		fullName = fmt.Sprintf("%s_%s_%s_%s", kindString(kind), ent.Type(), ent.Name(), name)
	}
	fullName = strings.ReplaceAll(fullName, " ", "")
	return newClient(ds.db, fullName)
}

func kindString(k component.Kind) string {
	switch k {
	case component.KindReceiver:
		return "receiver"
	case component.KindProcessor:
		return "processor"
	case component.KindExporter:
		return "exporter"
	case component.KindExtension:
		return "extension"
	default:
		return "other" // not expected
	}
}
