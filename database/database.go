// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package database

//go:generate mockery --name=^(Database)$ --output=../testutil --outpkg=testutil --case=snake

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

// Database is a database used to save offsets
type Database interface {
	Close() error
	Sync() error
	Update(func(*bbolt.Tx) error) error
	View(func(*bbolt.Tx) error) error
}

// StubDatabase is an implementation of Database that
// succeeds on all calls without persisting anything to disk.
// This is used when --database is unspecified.
type StubDatabase struct{}

// Close will be ignored by the stub database
func (d *StubDatabase) Close() error { return nil }

// Sync will be ignored by the stub database
func (d *StubDatabase) Sync() error { return nil }

// Update will be ignored by the stub database
func (d *StubDatabase) Update(func(tx *bbolt.Tx) error) error { return nil }

// View will be ignored by the stub database
func (d *StubDatabase) View(func(tx *bbolt.Tx) error) error { return nil }

// NewStubDatabase creates a new StubDatabase
func NewStubDatabase() *StubDatabase {
	return &StubDatabase{}
}

// OpenDatabase will open and create a database
func OpenDatabase(file string) (Database, error) {
	if file == "" {
		return NewStubDatabase(), nil
	}

	if _, err := os.Stat(filepath.Dir(file)); err != nil {
		if os.IsNotExist(err) {
			err := os.MkdirAll(filepath.Dir(file), 0755)
			if err != nil {
				return nil, fmt.Errorf("creating database directory: %s", err)
			}
		} else {
			return nil, err
		}
	}

	options := &bbolt.Options{Timeout: 1 * time.Second}
	return bbolt.Open(file, 0666, options)
}
