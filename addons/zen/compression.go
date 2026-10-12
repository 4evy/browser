package zen

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/pierrec/lz4/v4"
)

var (
	sessionHeader     = []byte("mozLz40\x00")
	errInvalidSession = errors.New("invalid or oversized Mozilla LZ4 session")
)

const maximumSessionSize = 64 << 20

// Mozilla stores a size-prefixed raw LZ4 block after its eight-byte header
// Bound the destination before handing the block to the LZ4 decoder
func decodeSession(encoded []byte) ([]byte, error) {
	if len(encoded) < 12 || len(encoded) > maximumSessionSize || !bytes.Equal(encoded[:8], sessionHeader) {
		return nil, errInvalidSession
	}
	size := int(binary.LittleEndian.Uint32(encoded[8:12]))
	if size > maximumSessionSize {
		return nil, errInvalidSession
	}
	output := make([]byte, size)
	n, err := lz4.UncompressBlock(encoded[12:], output)
	if err != nil || n != size {
		return nil, errInvalidSession
	}
	return output, nil
}

func encodeSession(data []byte) ([]byte, error) {
	if len(data) > maximumSessionSize {
		return nil, errInvalidSession
	}
	// A full CompressBlockBound buffer also encodes incompressible input
	output := make([]byte, 12+lz4.CompressBlockBound(len(data)))
	copy(output, sessionHeader)
	binary.LittleEndian.PutUint32(output[8:12], uint32(len(data)))
	var compressor lz4.Compressor
	n, err := compressor.CompressBlock(data, output[12:])
	if err != nil {
		return nil, err
	}
	if n+12 > maximumSessionSize {
		return nil, errInvalidSession
	}
	return output[:12+n], nil
}
