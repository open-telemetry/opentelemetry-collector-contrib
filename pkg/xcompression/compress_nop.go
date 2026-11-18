package xutilcompress

import "io"

// NopCompress copies input to output without compressing or decompressing it.
// It implements both CompressReader and CompressWriter for configurations
// where no compression is selected.
type NopCompress struct{}

var (
	_ CompressReader = (NopCompress{})
	_ CompressWriter = (NopCompress{})
)

// Compress copies src to dst and returns the number of bytes copied.
func (NopCompress) Compress(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// CompressBytes copies b to dst and returns the number of bytes written.
func (nop NopCompress) CompressBytes(dst io.Writer, b []byte) (int64, error) {
	return nop.fromBytes(dst, b)
}

// Decompress copies src to dst and returns the number of bytes copied.
func (NopCompress) Decompress(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// DecompressBytes copies b to dst and returns the number of bytes written.
func (nop NopCompress) DecompressBytes(dst io.Writer, b []byte) (int64, error) {
	return nop.fromBytes(dst, b)
}

func (NopCompress) fromBytes(dst io.Writer, b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	// Write directly to avoid allocating an intermediate reader to the heap.
	n, err := dst.Write(b)
	if err == nil && n != len(b) {
		err = io.ErrShortWrite
	}
	return int64(n), err
}
