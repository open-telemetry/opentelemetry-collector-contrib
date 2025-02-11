// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zstd

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

func resetTest() {
	TTL = time.Minute
	resetLibrary()
}

func TestCompressorNonNil(t *testing.T) {
	defer resetTest()

	for i := 1; i <= 10; i++ {
		require.NotNil(t, encoding.GetCompressor(fmt.Sprint(NamePrefix, i)))
	}
	require.Nil(t, encoding.GetCompressor(fmt.Sprint(NamePrefix, MinLevel-1)))
	require.Nil(t, encoding.GetCompressor(fmt.Sprint(NamePrefix, MaxLevel+1)))
}

func TestConfigLibraryError(t *testing.T) {
	defer resetTest()

	require.Error(t, SetEncoderConfig(EncoderConfig{
		Level:         1,
		WindowSizeMiB: 1024,
	}))
	require.NoError(t, SetDecoderConfig(DecoderConfig{
		MaxWindowSizeMiB: 1024,
	}))
}

func TestInvalidCompressorLevel(t *testing.T) {
	defer resetTest()

	require.Error(t, SetEncoderConfig(EncoderConfig{
		Level:         12,
		Concurrency:   10,
		WindowSizeMiB: 16,
	}))
}

func TestAllCompressorOptions(t *testing.T) {
	defer resetTest()

	require.NoError(t, SetEncoderConfig(EncoderConfig{
		Level:         9,
		Concurrency:   10,
		WindowSizeMiB: 16,
	}))
	require.NoError(t, SetDecoderConfig(DecoderConfig{
		Concurrency:      10,
		MaxWindowSizeMiB: 16,
		MemoryLimitMiB:   256,
	}))
}

func TestCompressorReset(t *testing.T) {
	defer resetTest()

	// Get compressor configs 1 and 2.
	comp1 := encoding.GetCompressor("zstdarrow1").(*combined)
	comp2 := encoding.GetCompressor("zstdarrow2").(*combined)

	// Get an object for level 1
	var buf bytes.Buffer
	wc, err := comp1.Compress(&buf)
	require.NoError(t, err)

	// Put back the once, it will be saved.
	save := wc.(*writer)
	require.NoError(t, wc.Close())
	require.Equal(t, 1, comp1.enc.pool.Size())
	require.Equal(t, 0, comp2.enc.pool.Size())

	// We get the same object pointer again.
	wc, err = comp1.Compress(&buf)
	require.NoError(t, err)
	require.Equal(t, save, wc.(*writer))

	// Modify 1's encoder configuration.
	encCfg1 := comp1.enc.getConfig()
	encCfg2 := comp2.enc.getConfig()
	cpyCfg1 := encCfg1
	cpyCfg1.WindowSizeMiB = 32

	require.Equal(t, Level(1), cpyCfg1.Level)
	require.NotEqual(t, cpyCfg1, encCfg1, "see %v %v", cpyCfg1, encCfg1)

	require.NoError(t, SetEncoderConfig(cpyCfg1))

	// The instances can't have changed.
	require.Equal(t, comp1, encoding.GetCompressor("zstdarrow1").(*combined))
	require.Equal(t, comp2, encoding.GetCompressor("zstdarrow2").(*combined))

	// Level 2 is unchanged
	require.Equal(t, encCfg2, comp2.enc.getConfig())

	// Level 1 is changed
	require.NotEqual(t, encCfg1, comp1.enc.getConfig(), "see %v %v", encCfg1, comp1.enc.getConfig())

	// Put back the saved item, it will not be placed back in the
	// pool due to reset.
	require.NoError(t, wc.Close())
	require.Equal(t, 0, comp1.enc.pool.Size())
	// Explicitly, we get a nil from the pool.
	v, _ := comp1.enc.pool.Get()
	require.Nil(t, v)
}

func TestDecompressorReset(t *testing.T) {
	defer resetTest()

	// Get compressor configs 3 and 4.
	comp3 := encoding.GetCompressor("zstdarrow3").(*combined)
	comp4 := encoding.GetCompressor("zstdarrow4").(*combined)

	// Get an object for level 3
	buf := new(bytes.Buffer)
	rd, err := comp3.Decompress(buf)
	require.NoError(t, err)
	_, err = rd.Read([]byte{})
	require.Error(t, err)

	// We get the same object pointer again.
	buf = new(bytes.Buffer)
	rd, err = comp3.Decompress(buf)
	require.NoError(t, err)
	_, err = rd.Read(nil)
	require.Error(t, err)

	// Modify 3's encoder configuration.
	decCfg3 := comp3.dec.getConfig()
	decCfg4 := comp4.dec.getConfig()
	cpyCfg3 := decCfg3
	cpyCfg3.MaxWindowSizeMiB = 128

	require.NotEqual(t, cpyCfg3, decCfg3, "see %v %v", cpyCfg3, decCfg3)

	require.NoError(t, SetDecoderConfig(cpyCfg3))

	// The instances can't have changed.
	require.Equal(t, comp3, encoding.GetCompressor("zstdarrow3").(*combined))
	require.Equal(t, comp4, encoding.GetCompressor("zstdarrow4").(*combined))

	// Level 4 is _also changed_
	require.Equal(t, cpyCfg3, comp4.dec.getConfig())
	require.NotEqual(t, decCfg4, comp4.dec.getConfig())

	// Level 3 is changed
	require.NotEqual(t, decCfg3, comp3.dec.getConfig(), "see %v %v", decCfg3, comp3.dec.getConfig())

	// Unlike the encoder test, which has an explicit Close() to its advantage,
	// we aren't testing the behavior of the finalizer that puts back into the MRU.
}

func TestGRPCCallOption(t *testing.T) {
	cfgN := func(l Level) EncoderConfig {
		return EncoderConfig{
			Level: l,
		}
	}
	cfg2 := cfgN(2)
	require.Equal(t, cfg2.Name(), cfg2.CallOption().(grpc.CompressorCallOption).CompressorType)

	cfgD := cfgN(DefaultLevel)
	cfg13 := cfgN(13)
	// Invalid maps to default call option
	require.Equal(t, cfgD.Name(), cfg13.CallOption().(grpc.CompressorCallOption).CompressorType)
}
