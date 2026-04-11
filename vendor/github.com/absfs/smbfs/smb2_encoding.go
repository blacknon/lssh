package smbfs

import (
	"encoding/binary"
	"unicode/utf16"
)

// SMB2 uses little-endian byte order for all multi-byte values
var le = binary.LittleEndian

// EncodeStringToUTF16LE encodes a Go string to UTF-16LE bytes (SMB wire format)
func EncodeStringToUTF16LE(s string) []byte {
	runes := utf16.Encode([]rune(s))
	buf := make([]byte, len(runes)*2)
	for i, r := range runes {
		le.PutUint16(buf[i*2:], r)
	}
	return buf
}

// DecodeUTF16LEToString decodes UTF-16LE bytes to a Go string
func DecodeUTF16LEToString(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	// Handle odd-length data by truncating
	if len(data)%2 != 0 {
		data = data[:len(data)-1]
	}
	runes := make([]uint16, len(data)/2)
	for i := range runes {
		runes[i] = le.Uint16(data[i*2:])
	}
	// Remove null terminator if present
	if len(runes) > 0 && runes[len(runes)-1] == 0 {
		runes = runes[:len(runes)-1]
	}
	return string(utf16.Decode(runes))
}

// PadTo8ByteBoundary returns the number of padding bytes needed to align to 8-byte boundary
func PadTo8ByteBoundary(offset int) int {
	remainder := offset % 8
	if remainder == 0 {
		return 0
	}
	return 8 - remainder
}

// AlignTo8 aligns a value up to the next 8-byte boundary
func AlignTo8(v int) int {
	return (v + 7) &^ 7
}

// ByteReader provides convenient methods for reading binary data
type ByteReader struct {
	data []byte
	pos  int
}

// NewByteReader creates a new ByteReader
func NewByteReader(data []byte) *ByteReader {
	return &ByteReader{data: data, pos: 0}
}

// Remaining returns the number of unread bytes
func (r *ByteReader) Remaining() int {
	return len(r.data) - r.pos
}

// Skip advances the position by n bytes
func (r *ByteReader) Skip(n int) {
	r.pos += n
}

// Seek sets the position
func (r *ByteReader) Seek(pos int) {
	r.pos = pos
}

// Position returns the current position
func (r *ByteReader) Position() int {
	return r.pos
}

// ReadBytes reads n bytes and advances position
func (r *ByteReader) ReadBytes(n int) []byte {
	if r.pos+n > len(r.data) {
		return nil
	}
	result := r.data[r.pos : r.pos+n]
	r.pos += n
	return result
}

// ReadOneByte reads a single byte (named to avoid conflict with io.ByteReader)
func (r *ByteReader) ReadOneByte() byte {
	if r.pos >= len(r.data) {
		return 0
	}
	b := r.data[r.pos]
	r.pos++
	return b
}

// ReadUint16 reads a little-endian uint16
func (r *ByteReader) ReadUint16() uint16 {
	if r.pos+2 > len(r.data) {
		return 0
	}
	v := le.Uint16(r.data[r.pos:])
	r.pos += 2
	return v
}

// ReadUint32 reads a little-endian uint32
func (r *ByteReader) ReadUint32() uint32 {
	if r.pos+4 > len(r.data) {
		return 0
	}
	v := le.Uint32(r.data[r.pos:])
	r.pos += 4
	return v
}

// ReadUint64 reads a little-endian uint64
func (r *ByteReader) ReadUint64() uint64 {
	if r.pos+8 > len(r.data) {
		return 0
	}
	v := le.Uint64(r.data[r.pos:])
	r.pos += 8
	return v
}

// ReadFileID reads a 16-byte FileID
func (r *ByteReader) ReadFileID() FileID {
	return FileID{
		Persistent: r.ReadUint64(),
		Volatile:   r.ReadUint64(),
	}
}

// ReadGUID reads a 16-byte GUID
func (r *ByteReader) ReadGUID() [16]byte {
	var guid [16]byte
	copy(guid[:], r.ReadBytes(16))
	return guid
}

// ReadUTF16String reads a UTF-16LE string of specified byte length
func (r *ByteReader) ReadUTF16String(byteLen int) string {
	data := r.ReadBytes(byteLen)
	return DecodeUTF16LEToString(data)
}

// ByteWriter provides convenient methods for writing binary data
type ByteWriter struct {
	data []byte
}

// NewByteWriter creates a new ByteWriter with initial capacity
func NewByteWriter(capacity int) *ByteWriter {
	return &ByteWriter{data: make([]byte, 0, capacity)}
}

// Bytes returns the written bytes
func (w *ByteWriter) Bytes() []byte {
	return w.data
}

// Len returns the number of written bytes
func (w *ByteWriter) Len() int {
	return len(w.data)
}

// Reset clears all written data
func (w *ByteWriter) Reset() {
	w.data = w.data[:0]
}

// WriteBytes appends raw bytes
func (w *ByteWriter) WriteBytes(b []byte) {
	w.data = append(w.data, b...)
}

// WriteOneByte appends a single byte (named to avoid conflict with io.ByteWriter)
func (w *ByteWriter) WriteOneByte(b byte) {
	w.data = append(w.data, b)
}

// WriteUint16 appends a little-endian uint16
func (w *ByteWriter) WriteUint16(v uint16) {
	var buf [2]byte
	le.PutUint16(buf[:], v)
	w.data = append(w.data, buf[:]...)
}

// WriteUint32 appends a little-endian uint32
func (w *ByteWriter) WriteUint32(v uint32) {
	var buf [4]byte
	le.PutUint32(buf[:], v)
	w.data = append(w.data, buf[:]...)
}

// WriteUint64 appends a little-endian uint64
func (w *ByteWriter) WriteUint64(v uint64) {
	var buf [8]byte
	le.PutUint64(buf[:], v)
	w.data = append(w.data, buf[:]...)
}

// WriteFileID appends a 16-byte FileID
func (w *ByteWriter) WriteFileID(f FileID) {
	w.WriteUint64(f.Persistent)
	w.WriteUint64(f.Volatile)
}

// WriteGUID appends a 16-byte GUID
func (w *ByteWriter) WriteGUID(guid [16]byte) {
	w.data = append(w.data, guid[:]...)
}

// WriteUTF16String appends a UTF-16LE encoded string
func (w *ByteWriter) WriteUTF16String(s string) {
	w.WriteBytes(EncodeStringToUTF16LE(s))
}

// WriteZeros appends n zero bytes
func (w *ByteWriter) WriteZeros(n int) {
	for i := 0; i < n; i++ {
		w.data = append(w.data, 0)
	}
}

// WritePadTo8 pads to 8-byte boundary
func (w *ByteWriter) WritePadTo8() {
	pad := PadTo8ByteBoundary(len(w.data))
	w.WriteZeros(pad)
}

// SetUint16At writes a uint16 at a specific position (for backpatching)
func (w *ByteWriter) SetUint16At(pos int, v uint16) {
	if pos+2 <= len(w.data) {
		le.PutUint16(w.data[pos:], v)
	}
}

// SetUint32At writes a uint32 at a specific position (for backpatching)
func (w *ByteWriter) SetUint32At(pos int, v uint32) {
	if pos+4 <= len(w.data) {
		le.PutUint32(w.data[pos:], v)
	}
}

// GUID utilities

// NewGUID creates a new random GUID
func NewGUID() [16]byte {
	// Use crypto/rand for proper randomness in production
	// For now, this is a placeholder that should be replaced
	var guid [16]byte
	// TODO: Use crypto/rand to generate proper GUID
	return guid
}

// GUIDToString converts a GUID to string format (XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX)
func GUIDToString(guid [16]byte) string {
	return string([]byte{
		hexDigit(guid[3]>>4), hexDigit(guid[3]&0xf),
		hexDigit(guid[2]>>4), hexDigit(guid[2]&0xf),
		hexDigit(guid[1]>>4), hexDigit(guid[1]&0xf),
		hexDigit(guid[0]>>4), hexDigit(guid[0]&0xf),
		'-',
		hexDigit(guid[5]>>4), hexDigit(guid[5]&0xf),
		hexDigit(guid[4]>>4), hexDigit(guid[4]&0xf),
		'-',
		hexDigit(guid[7]>>4), hexDigit(guid[7]&0xf),
		hexDigit(guid[6]>>4), hexDigit(guid[6]&0xf),
		'-',
		hexDigit(guid[8]>>4), hexDigit(guid[8]&0xf),
		hexDigit(guid[9]>>4), hexDigit(guid[9]&0xf),
		'-',
		hexDigit(guid[10]>>4), hexDigit(guid[10]&0xf),
		hexDigit(guid[11]>>4), hexDigit(guid[11]&0xf),
		hexDigit(guid[12]>>4), hexDigit(guid[12]&0xf),
		hexDigit(guid[13]>>4), hexDigit(guid[13]&0xf),
		hexDigit(guid[14]>>4), hexDigit(guid[14]&0xf),
		hexDigit(guid[15]>>4), hexDigit(guid[15]&0xf),
	})
}

func hexDigit(b byte) byte {
	if b < 10 {
		return '0' + b
	}
	return 'a' + b - 10
}
