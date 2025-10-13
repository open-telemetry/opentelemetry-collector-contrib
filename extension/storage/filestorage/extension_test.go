// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filestorage

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/extension/extensiontest"
	"go.opentelemetry.io/collector/extension/xextension/storage"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestExtensionIntegrity(t *testing.T) {
	ctx := t.Context()
	se := newTestExtension(t)

	type mockComponent struct {
		kind component.Kind
		name component.ID
	}

	components := []mockComponent{
		{kind: component.KindReceiver, name: newTestEntity("receiver_one")},
		{kind: component.KindReceiver, name: newTestEntity("receiver_two")},
		{kind: component.KindProcessor, name: newTestEntity("processor_one")},
		{kind: component.KindProcessor, name: newTestEntity("processor_two")},
		{kind: component.KindExporter, name: newTestEntity("exporter_one")},
		{kind: component.KindExporter, name: newTestEntity("exporter_two")},
		{kind: component.KindExtension, name: newTestEntity("extension_one")},
		{kind: component.KindExtension, name: newTestEntity("extension_two")},
	}

	// Make a client for each component
	clients := make(map[component.ID]storage.Client)
	for _, c := range components {
		client, err := se.GetClient(ctx, c.kind, c.name, "")
		require.NoError(t, err)
		t.Cleanup(func() {
			require.NoError(t, client.Close(ctx))
		})

		clients[c.name] = client
	}

	thrashClient := func(wg *sync.WaitGroup, n component.ID, c storage.Client) {
		// keys and values
		keys := []string{"a", "b", "c", "d", "e"}
		myBytes := []byte(n.Name())

		// Set my values
		for i := range keys {
			err := c.Set(ctx, keys[i], myBytes)
			require.NoError(t, err)
		}

		// Repeatedly thrash client
		for range 100 {
			// Make sure my values are still mine
			for i := range keys {
				v, err := c.Get(ctx, keys[i])
				require.NoError(t, err)
				require.Equal(t, myBytes, v)
			}

			// Delete my values
			for i := range keys {
				err := c.Delete(ctx, keys[i])
				require.NoError(t, err)
			}

			// Reset my values
			for i := range keys {
				err := c.Set(ctx, keys[i], myBytes)
				require.NoError(t, err)
			}
		}
		wg.Done()
	}

	// Use clients concurrently
	var wg sync.WaitGroup
	for name, client := range clients {
		wg.Add(1)
		go thrashClient(&wg, name, client)
	}
	wg.Wait()
}

func TestClientHandlesSimpleCases(t *testing.T) {
	ctx := t.Context()
	se := newTestExtension(t)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)

	myBytes := []byte("value")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// Set the data
	err = client.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	// Set it again (nop does not error)
	err = client.Set(ctx, "key", myBytes)
	require.NoError(t, err)

	// Get actual data
	data, err := client.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes, data)

	// Delete the data
	err = client.Delete(ctx, "key")
	require.NoError(t, err)

	// Delete it again (nop does not error)
	err = client.Delete(ctx, "key")
	require.NoError(t, err)

	// Get missing data
	data, err = client.Get(ctx, "key")
	require.NoError(t, err)
	require.Nil(t, data)
}

func TestTwoClientsWithDifferentNames(t *testing.T) {
	ctx := t.Context()
	se := newTestExtension(t)

	client1, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"foo",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client1.Close(ctx))
	})

	client2, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"bar",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client2.Close(ctx))
	})

	myBytes1 := []byte("value1")
	myBytes2 := []byte("value2")

	// Set the data
	err = client1.Set(ctx, "key", myBytes1)
	require.NoError(t, err)

	err = client2.Set(ctx, "key", myBytes2)
	require.NoError(t, err)

	// Check it was associated accordingly
	data, err := client1.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes1, data)

	data, err = client2.Get(ctx, "key")
	require.NoError(t, err)
	require.Equal(t, myBytes2, data)
}

func TestSanitize(t *testing.T) {
	testCases := []struct {
		name          string
		componentName string
		sanitizedName string
	}{
		{
			name:          "safe characters",
			componentName: `.UPPERCASE-lowercase_1234567890`,
			sanitizedName: `.UPPERCASE-lowercase_1234567890`,
		},
		{
			name:          "name with a slash",
			componentName: `logs/json`,
			sanitizedName: `logs~002Fjson`,
		},
		{
			name:          "name with a tilde",
			componentName: `logs~json`,
			sanitizedName: `logs~007Ejson`,
		},
		{
			name:          "popular unsafe characters",
			componentName: `tilde~slash/backslash\colon:asterisk*questionmark?quote'doublequote"angle<>pipe|exclamationmark!percent%space `,
			sanitizedName: `tilde~007Eslash~002Fbackslash~005Ccolon~003Aasterisk~002Aquestionmark~003Fquote~0027doublequote~0022angle~003C~003Epipe~007Cexclamationmark~0021percent~0025space~0020`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.sanitizedName, sanitize(testCase.componentName))
		})
	}
}

func TestComponentNameWithUnsafeCharacters(t *testing.T) {
	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	client, err := se.GetClient(
		t.Context(),
		component.KindReceiver,
		newTestEntity("my/slashed/component*"),
		"",
	)

	require.NoError(t, err)
	require.NotNil(t, client)

	client.Close(t.Context())
}

func TestGetClientErrorsOnDeletedDirectory(t *testing.T) {
	ctx := t.Context()

	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	// Delete the directory before getting client
	err = os.RemoveAll(tempDir)
	require.NoError(t, err)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)

	require.Error(t, err)
	require.Nil(t, client)
}

func newTestExtension(t *testing.T) storage.Extension {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = t.TempDir()

	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	return se
}

func newTestEntity(name string) component.ID {
	return component.MustNewIDWithName("nop", name)
}

func TestCompaction(t *testing.T) {
	ctx := t.Context()

	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)

	file := files[0]
	path := filepath.Join(tempDir, file.Name())
	stats, err := os.Stat(path)
	require.NoError(t, err)

	var key string
	var i int

	// magic numbers giving enough data to force bbolt to allocate a new page
	// see https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/9004 for some discussion
	numEntries := 50
	entrySize := 512
	entry := make([]byte, entrySize)

	// add the data to the db
	for i = 0; i < numEntries; i++ {
		key = fmt.Sprintf("key_%d", i)
		err = client.Set(ctx, key, entry)
		require.NoError(t, err)
	}

	// compact the db
	c, ok := client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(tempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check size after compaction
	newStats, err := os.Stat(path)
	require.NoError(t, err)
	require.Less(t, stats.Size(), newStats.Size())

	// remove data from database
	for i = range numEntries {
		key = fmt.Sprintf("key_%d", i)
		err = c.Delete(ctx, key)
		require.NoError(t, err)
	}

	// compact after data removal
	c, ok = client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(tempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check size
	stats = newStats
	newStats, err = os.Stat(path)
	require.NoError(t, err)
	require.Less(t, newStats.Size(), stats.Size())
}

// TestCompactionRemoveTemp validates if temporary db used for compaction is removed afterwards
// test is performed for both: the same and different than storage directories
func TestCompactionRemoveTemp(t *testing.T) {
	ctx := t.Context()

	tempDir := t.TempDir()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir

	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check if only db exists in tempDir
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	fileName := files[0].Name()

	// perform compaction in the same directory
	c, ok := client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(tempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check if only db exists in tempDir
	files, err = os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, fileName, files[0].Name())

	// perform compaction in different directory
	emptyTempDir := t.TempDir()

	c, ok = client.(*fileStorageClient)
	require.True(t, ok)
	err = c.Compact(emptyTempDir, cfg.Timeout, 1)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	// check if emptyTempDir is empty after compaction
	files, err = os.ReadDir(emptyTempDir)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestCleanupOnStart(t *testing.T) {
	ctx := t.Context()

	tempDir := t.TempDir()
	// simulate left temporary compaction file from killed process
	temp, _ := os.CreateTemp(tempDir, TempDbPrefix)
	temp.Close()

	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir
	cfg.Compaction.Directory = tempDir
	cfg.Compaction.CleanupOnStart = true
	extension, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)
	require.NoError(t, se.Start(ctx, componenttest.NewNopHost()))

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close(ctx))
	})

	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	require.Len(t, files, 1)
}

func TestCompactionOnStart(t *testing.T) {
	ctx := t.Context()
	f := NewFactory()

	logCore, logObserver := observer.New(zap.DebugLevel)
	logger := zap.New(logCore)
	set := extensiontest.NewNopSettings(f.Type())
	set.Logger = logger

	tempDir := t.TempDir()
	temp, _ := os.CreateTemp(tempDir, TempDbPrefix)
	temp.Close()

	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Directory = tempDir
	cfg.Compaction.Directory = tempDir
	cfg.Compaction.OnStart = true
	extension, err := f.Create(t.Context(), set, cfg)
	require.NoError(t, err)

	se, ok := extension.(storage.Extension)
	require.True(t, ok)
	require.NoError(t, se.Start(ctx, componenttest.NewNopHost()))

	client, err := se.GetClient(
		ctx,
		component.KindReceiver,
		newTestEntity("my_component"),
		"",
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		// At least one compaction should have happened on start
		require.GreaterOrEqual(t, len(logObserver.FilterMessage("finished compaction").All()), 1)
		require.NoError(t, client.Close(t.Context()))
	})
}

func TestDirectoryCreation(t *testing.T) {
	tests := []struct {
		name     string
		config   func(*testing.T, extension.Factory) *Config
		validate func(*testing.T, *Config)
	}{
		{
			name: "create directory true - no error",
			config: func(t *testing.T, f extension.Factory) *Config {
				tempDir := t.TempDir()
				storageDir := filepath.Join(tempDir, uuid.NewString())
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = storageDir
				cfg.CreateDirectory = true
				cfg.DirectoryPermissions = "0750"
				require.NoError(t, cfg.Validate())
				return cfg
			},
			validate: func(t *testing.T, cfg *Config) {
				require.DirExists(t, cfg.Directory)
				s, err := os.Stat(cfg.Directory)
				require.NoError(t, err)
				var expectedFileMode os.FileMode
				if runtime.GOOS == "windows" { // on Windows, we get 0777 for writable directories
					expectedFileMode = os.FileMode(0o777)
				} else {
					expectedFileMode = os.FileMode(0o750)
				}
				require.Equal(t, expectedFileMode, s.Mode()&os.ModePerm)
			},
		},
		{
			name: "create directory true - no error - 0700 permissions",
			config: func(t *testing.T, f extension.Factory) *Config {
				tempDir := t.TempDir()
				storageDir := filepath.Join(tempDir, uuid.NewString())
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = storageDir
				cfg.DirectoryPermissions = "0700"
				cfg.CreateDirectory = true
				require.NoError(t, cfg.Validate())
				return cfg
			},
			validate: func(t *testing.T, cfg *Config) {
				require.DirExists(t, cfg.Directory)
				s, err := os.Stat(cfg.Directory)
				require.NoError(t, err)
				var expectedFileMode os.FileMode
				if runtime.GOOS == "windows" { // on Windows, we get 0777 for writable directories
					expectedFileMode = os.FileMode(0o777)
				} else {
					expectedFileMode = os.FileMode(0o700)
				}
				require.Equal(t, expectedFileMode, s.Mode()&os.ModePerm)
			},
		},
		{
			name: "create directory false - error",
			config: func(t *testing.T, f extension.Factory) *Config {
				tempDir := t.TempDir()
				storageDir := filepath.Join(tempDir, uuid.NewString())
				cfg := f.CreateDefaultConfig().(*Config)
				cfg.Directory = storageDir
				cfg.CreateDirectory = false
				require.ErrorIs(t, cfg.Validate(), os.ErrNotExist)
				return cfg
			},
			validate: func(t *testing.T, cfg *Config) {
				require.NoDirExists(t, cfg.Directory)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewFactory()
			config := tt.config(t, f)
			if config != nil {
				ext, err := f.Create(t.Context(), extensiontest.NewNopSettings(f.Type()), config)
				require.NoError(t, err)
				require.NotNil(t, ext)
				tt.validate(t, config)
			}
		})
	}
}

func TestRecreate(t *testing.T) {
	ctx := t.Context()
	temp := t.TempDir()
	f := NewFactory()

	config := f.CreateDefaultConfig().(*Config)
	config.Directory = temp

	// step 1: create an extension with default config and write some data
	{
		ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), config)
		require.NoError(t, err)
		require.NotNil(t, ext)

		se, ok := ext.(storage.Extension)
		require.True(t, ok)

		client, err := se.GetClient(ctx, component.KindReceiver, component.MustNewID("filelog"), "")
		require.NoError(t, err)
		require.NotNil(t, client)

		// write the data and make sure it is set in the subsequent get.
		require.NoError(t, client.Set(ctx, "key", []byte("val")))
		val, err := client.Get(ctx, "key")
		require.Equal(t, val, []byte("val"))
		require.NoError(t, err)

		// close the extension
		require.NoError(t, client.Close(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	}

	// step 2: re-create the extension to make sure that the data is therw
	{
		ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), config)
		require.NoError(t, err)
		require.NotNil(t, ext)
		se, ok := ext.(storage.Extension)
		require.True(t, ok)

		client, err := se.GetClient(ctx, component.KindReceiver, component.MustNewID("filelog"), "")
		require.NoError(t, err)
		require.NotNil(t, client)

		// make sure that the data exists from the previous pass.
		val, err := client.Get(ctx, "key")
		require.Equal(t, val, []byte("val"))
		require.NoError(t, err)

		// close the extension
		require.NoError(t, client.Close(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	}

	// step 3: re-create the extension, but with Recreate=true and make sure that the data still exists
	// (since recreate now only happens on panic, not always when recreate=true)
	{
		config.Recreate = true
		ext, err := f.Create(ctx, extensiontest.NewNopSettings(f.Type()), config)
		require.NoError(t, err)
		require.NotNil(t, ext)
		se, ok := ext.(storage.Extension)
		require.True(t, ok)

		client, err := se.GetClient(ctx, component.KindReceiver, component.MustNewID("filelog"), "")
		require.NoError(t, err)
		require.NotNil(t, client)

		// The data should still exist since no panic occurred
		val, err := client.Get(ctx, "key")
		require.Equal(t, val, []byte("val"))
		require.NoError(t, err)

		// close the extension
		require.NoError(t, client.Close(ctx))
		require.NoError(t, ext.Shutdown(ctx))
	}
}
