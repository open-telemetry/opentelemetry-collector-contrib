package fileexporter

import "github.com/klauspost/compress/zstd"

// compressFunc defines how to compress encoded telemetry data.
type compressFunc func(src []byte) []byte

var encoder, _ = zstd.NewWriter(nil)

var compressFuncs = map[string]compressFunc{
	defaultCompression: noneCompress,
	compressionZSTD:    zstdCompress,
}

// zstdCompress compress a buffer with zstd
func zstdCompress(src []byte) []byte {
	return encoder.EncodeAll(src, make([]byte, 0, len(src)))
}

// noneCompress return src
func noneCompress(src []byte) []byte {
	return src
}
