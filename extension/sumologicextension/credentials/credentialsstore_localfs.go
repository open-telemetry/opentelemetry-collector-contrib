// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package credentials

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/hashicorp/go-multierror"
	"go.uber.org/zap"
)

const (
	DefaultCollectorCredentialsDirectory = ".sumologic-otel-collector/"
)

func GetDefaultCollectorCredentialsDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return path.Join(home, DefaultCollectorCredentialsDirectory), nil
}

// LocalFsStore implements Store interface and can be used to store and retrieve
// collector credentials from local file system.
//
// Files are stored locally in collectorCredentialsDirectory.
type LocalFsStore struct {
	collectorCredentialsDirectory string
	logger                        *zap.Logger
}

type LocalFsStoreOpt func(*LocalFsStore)

func WithLogger(l *zap.Logger) LocalFsStoreOpt {
	return func(s *LocalFsStore) {
		s.logger = l
	}
}

func WithCredentialsDirectory(dir string) LocalFsStoreOpt {
	return func(s *LocalFsStore) {
		s.collectorCredentialsDirectory = dir
	}
}

func NewLocalFsStore(opts ...LocalFsStoreOpt) (Store, error) {
	defaultDir, err := GetDefaultCollectorCredentialsDirectory()
	if err != nil {
		return nil, err
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}

	store := LocalFsStore{
		collectorCredentialsDirectory: defaultDir,
		logger:                        logger,
	}
	for _, opt := range opts {
		opt(&store)
	}

	return store, err
}

// Check checks if collector credentials can be found under a name being a hash
// of provided key inside collectorCredentialsDirectory.
func (cr LocalFsStore) Check(key string) bool {
	f := func(hasher Hasher, key string) bool {
		filenameHash, err := HashKeyToFilename(key)
		if err != nil {
			return false
		}
		path := path.Join(cr.collectorCredentialsDirectory, filenameHash)
		if _, err := os.Stat(path); err != nil {
			return false
		}
		return true
	}

	if f(_getHasher(), key) {
		return true
	}
	if f(_getDeprecatedHasher(), key) {
		return true
	}

	return false
}

// Get retrieves collector credentials stored in local file system and then
// decrypts it using a hash of provided key.
func (cr LocalFsStore) Get(key string) (CollectorCredentials, error) {
	f := func(hasher Hasher, key string) (CollectorCredentials, error) {
		filenameHash, err := HashKeyToFilename(key)
		if err != nil {
			return CollectorCredentials{}, err
		}

		path := path.Join(cr.collectorCredentialsDirectory, filenameHash)
		creds, err := os.Open(path)
		if err != nil {
			return CollectorCredentials{}, err
		}
		defer creds.Close()

		encryptedCreds, err := io.ReadAll(creds)
		if err != nil {
			return CollectorCredentials{}, err
		}

		encKey, err := HashKeyToEncryptionKey(key)
		if err != nil {
			return CollectorCredentials{}, err
		}

		collectorCreds, err := decrypt(encryptedCreds, encKey)
		if err != nil {
			return CollectorCredentials{}, err
		}

		var credentialsInfo CollectorCredentials
		if err = json.Unmarshal(collectorCreds, &credentialsInfo); err != nil {
			return CollectorCredentials{}, err
		}

		cr.logger.Info("Collector registration credentials retrieved from local fs",
			zap.String("path", path),
		)

		return credentialsInfo, nil
	}

	if creds, err := f(_getHasher(), key); err == nil {
		return creds, nil
	}

	creds, err := f(_getDeprecatedHasher(), key)
	if err == nil {
		return creds, nil
	}
	return CollectorCredentials{}, err
}

// Store stores collector credentials in a file in directory as specified
// in CollectorCredentialsDirectory.
// The credentials are encrypted using the provided key.
func (cr LocalFsStore) Store(key string, creds CollectorCredentials) error {
	if err := ensureDir(cr.collectorCredentialsDirectory); err != nil {
		return err
	}

	f := func(hasher Hasher, key string, creds CollectorCredentials) error {
		filenameHash, err := HashKeyToFilename(key)
		if err != nil {
			return err
		}
		path := path.Join(cr.collectorCredentialsDirectory, filenameHash)
		collectorCreds, err := json.Marshal(creds)
		if err != nil {
			return fmt.Errorf("failed marshalling collector credentials: %w", err)
		}

		encKey, err := HashKeyToEncryptionKey(key)
		if err != nil {
			return err
		}

		encryptedCreds, err := encrypt(collectorCreds, encKey)
		if err != nil {
			return err
		}

		if err = os.WriteFile(path, encryptedCreds, 0600); err != nil {
			return fmt.Errorf("failed to save credentials file '%s': %w",
				path, err,
			)
		}

		cr.logger.Info("Collector registration credentials stored locally",
			zap.String("path", path),
		)

		return nil
	}

	if err := f(_getHasher(), key, creds); err == nil {
		return nil
	}

	err := f(_getDeprecatedHasher(), key, creds)
	if err == nil {
		return nil
	}
	return err
}

func (cr LocalFsStore) Delete(key string) error {
	f := func(hasher Hasher, key string) error {
		filenameHash, err := HashKeyToFilenameWith(hasher, key)
		if err != nil {
			return err
		}

		path := path.Join(cr.collectorCredentialsDirectory, filenameHash)

		if _, err := os.Stat(path); err != nil {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("failed to remove credentials file '%s': %w",
				path, err,
			)
		}

		cr.logger.Debug("Collector registration credentials removed",
			zap.String("path", path),
		)

		return nil
	}

	var errResult error
	if err := f(_getHasher(), key); err != nil {
		errResult = multierror.Append(errResult, err)
	}
	if err := f(_getDeprecatedHasher(), key); err != nil {
		errResult = multierror.Append(errResult, err)
	}

	return errResult
}

// Validate checks if the store is operating correctly
// This mostly means file permissions and the like
func (cr LocalFsStore) Validate() error {
	if err := ensureDir(cr.collectorCredentialsDirectory); err != nil {
		return err
	}

	return nil
}

// ensureDir checks if the specified directory exists and has the right permissions
// if it doesn't then it tries to create it.
func ensureDir(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		if err := os.Mkdir(path, 0700); err != nil {
			return err
		}
		return nil
	}

	// If the directory doesn't have the execution bit then
	// set it so that we can 'exec' into it.
	if fi.Mode().Perm() != 0700 {
		if err := os.Chmod(path, 0700); err != nil {
			return err
		}
	}

	return nil
}
