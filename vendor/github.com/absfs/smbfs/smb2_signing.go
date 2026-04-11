package smbfs

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"log"
)

// SMB2 signature field is at offset 48 in the SMB2 header (16 bytes)
const (
	SignatureOffset = 48
	SignatureLength = 16
)

// SMB2_FLAGS_SIGNED is defined in smb2_types.go

// SignMessage signs an SMB2 message using the appropriate algorithm for the dialect
// Returns the signature bytes (16 bytes) to be placed in the header
func SignMessage(message []byte, signingKey []byte, dialect SMBDialect) []byte {
	if len(signingKey) == 0 || len(message) < SMB2HeaderSize {
		return nil
	}

	// Create a copy of the message with signature field zeroed
	msgCopy := make([]byte, len(message))
	copy(msgCopy, message)

	// Zero out the signature field (16 bytes at offset 48)
	for i := SignatureOffset; i < SignatureOffset+SignatureLength; i++ {
		msgCopy[i] = 0
	}

	var signature []byte

	if dialect >= SMB3_0 {
		// SMB 3.x: Use AES-128-CMAC
		signature = computeAESCMAC(msgCopy, signingKey)
	} else {
		// SMB 2.x: Use HMAC-SHA256
		signature = computeHMACSHA256(msgCopy, signingKey)
	}

	// Debug logging
	log.Printf("[DEBUG] SignMessage: dialect=%s, keyLen=%d, msgLen=%d, sigLen=%d",
		dialect.String(), len(signingKey), len(message), len(signature))
	if len(signingKey) >= 16 {
		log.Printf("[DEBUG] SignMessage: signingKey=%x (first 16)", signingKey[:16])
	}
	if len(signature) >= 16 {
		log.Printf("[DEBUG] SignMessage: signature=%x", signature[:16])
	}

	return signature
}

// VerifySignature verifies an SMB2 message signature
func VerifySignature(message []byte, signingKey []byte, dialect SMBDialect) bool {
	if len(signingKey) == 0 || len(message) < SMB2HeaderSize {
		return false
	}

	// Extract the existing signature
	existingSig := make([]byte, SignatureLength)
	copy(existingSig, message[SignatureOffset:SignatureOffset+SignatureLength])

	// Compute expected signature
	expectedSig := SignMessage(message, signingKey, dialect)
	if expectedSig == nil {
		return false
	}

	// Compare signatures
	return hmac.Equal(existingSig, expectedSig)
}

// computeHMACSHA256 computes HMAC-SHA256 and returns first 16 bytes
func computeHMACSHA256(message []byte, key []byte) []byte {
	// Ensure key is 16 bytes (pad or truncate)
	signingKey := make([]byte, 16)
	copy(signingKey, key)

	h := hmac.New(sha256.New, signingKey)
	h.Write(message)
	hash := h.Sum(nil)

	// Return first 16 bytes of the 32-byte hash
	return hash[:16]
}

// computeAESCMAC computes AES-128-CMAC as per RFC 4493
func computeAESCMAC(message []byte, key []byte) []byte {
	// Ensure key is 16 bytes
	signingKey := make([]byte, 16)
	copy(signingKey, key)

	block, err := aes.NewCipher(signingKey)
	if err != nil {
		return nil
	}

	// Generate subkeys K1 and K2
	k1, k2 := generateCMACSubkeys(block)

	// Process message
	n := (len(message) + 15) / 16 // Number of blocks
	if n == 0 {
		n = 1
	}

	// Prepare the last block
	lastBlockComplete := len(message) > 0 && len(message)%16 == 0
	var lastBlock []byte

	if lastBlockComplete {
		// Last block is complete - XOR with K1
		lastBlock = make([]byte, 16)
		copy(lastBlock, message[(n-1)*16:])
		xorBytes(lastBlock, k1)
	} else {
		// Last block is incomplete - pad and XOR with K2
		lastBlock = make([]byte, 16)
		remaining := len(message) % 16
		if len(message) > 0 {
			copy(lastBlock, message[(n-1)*16:])
		}
		lastBlock[remaining] = 0x80 // Padding: 10000...
		xorBytes(lastBlock, k2)
	}

	// CBC-MAC computation
	x := make([]byte, 16) // Zero IV

	// Process all blocks except the last
	for i := 0; i < n-1; i++ {
		blockData := message[i*16 : (i+1)*16]
		xorBytes(x, blockData)
		block.Encrypt(x, x)
	}

	// Process the last block
	xorBytes(x, lastBlock)
	block.Encrypt(x, x)

	return x
}

// generateCMACSubkeys generates K1 and K2 subkeys for AES-CMAC
func generateCMACSubkeys(block cipher.Block) (k1, k2 []byte) {
	const rb = 0x87 // R_b for 128-bit blocks

	// L = AES(K, 0^128)
	l := make([]byte, 16)
	block.Encrypt(l, l)

	// K1 = L << 1
	k1 = make([]byte, 16)
	shiftLeft(k1, l)
	if l[0]&0x80 != 0 {
		k1[15] ^= rb
	}

	// K2 = K1 << 1
	k2 = make([]byte, 16)
	shiftLeft(k2, k1)
	if k1[0]&0x80 != 0 {
		k2[15] ^= rb
	}

	return k1, k2
}

// shiftLeft performs a left shift by 1 bit on a byte slice
func shiftLeft(dst, src []byte) {
	overflow := byte(0)
	for i := len(src) - 1; i >= 0; i-- {
		newOverflow := src[i] >> 7
		dst[i] = (src[i] << 1) | overflow
		overflow = newOverflow
	}
}

// xorBytes XORs src into dst (dst = dst XOR src)
func xorBytes(dst, src []byte) {
	for i := 0; i < len(dst) && i < len(src); i++ {
		dst[i] ^= src[i]
	}
}

// DeriveSigningKey derives the signing key for SMB 3.x using SP800-108 KDF
// For SMB 3.0/3.0.2: SigningKey = KDF(SessionKey, "SMB2AESCMAC\0", "SmbSign\0")
// For SMB 3.1.1: SigningKey = KDF(SessionKey, "SMBSigningKey\0", PreauthIntegrityHash)
func DeriveSigningKey(sessionKey []byte, dialect SMBDialect, preauthHash []byte) []byte {
	if dialect < SMB3_0 {
		// SMB 2.x uses session key directly
		log.Printf("[DEBUG] DeriveSigningKey: SMB 2.x - using session key directly")
		return sessionKey
	}

	var label, context []byte

	if dialect >= SMB3_1_1 && len(preauthHash) > 0 {
		// SMB 3.1.1 uses different label and preauth hash as context
		label = []byte("SMBSigningKey\x00")
		context = preauthHash
		log.Printf("[DEBUG] DeriveSigningKey: SMB 3.1.1 - full preauthHash=%x (len=%d)",
			preauthHash, len(preauthHash))
	} else {
		// SMB 3.0/3.0.2 (or SMB 3.1.1 without preauthHash)
		label = []byte("SMB2AESCMAC\x00")
		context = []byte("SmbSign\x00")
		log.Printf("[DEBUG] DeriveSigningKey: SMB 3.0 style - preauthHash len=%d", len(preauthHash))
	}

	signingKey := kdfSP800108(sessionKey, label, context, 16)
	log.Printf("[DEBUG] DeriveSigningKey: sessionKey=%x signingKey=%x", sessionKey, signingKey)
	return signingKey
}

// kdfSP800108 implements the SP800-108 KDF in Counter Mode with HMAC-SHA256
// This generates a key of the specified length in bytes
func kdfSP800108(ki, label, context []byte, lengthBytes int) []byte {
	// Per MS-SMB2 section 3.1.4.2:
	// PRF = HMAC-SHA256
	// r = 32 (counter is 4 bytes / 32 bits)
	// L = output length in bits

	lengthBits := uint32(lengthBytes * 8)

	// Output key
	result := make([]byte, 0, lengthBytes)

	// Counter starts at 1
	counter := uint32(1)

	for len(result) < lengthBytes {
		// K(i) = PRF(KI, [i]_2 || Label || 0x00 || Context || [L]_2)
		h := hmac.New(sha256.New, ki)

		// [i]_2 - counter as 4 bytes big-endian
		counterBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(counterBytes, counter)
		h.Write(counterBytes)

		// Label (includes null terminator per spec)
		h.Write(label)

		// 0x00 separator between label and context (per SP800-108)
		h.Write([]byte{0x00})

		// Context
		h.Write(context)

		// [L]_2 - output length in bits as 4 bytes big-endian
		lengthBitsBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(lengthBitsBytes, lengthBits)
		h.Write(lengthBitsBytes)

		// Append PRF output
		result = append(result, h.Sum(nil)...)
		counter++
	}

	// Truncate to requested length
	return result[:lengthBytes]
}

// ApplySignature applies a signature to an SMB2 message
// It modifies the message in place
func ApplySignature(message []byte, signature []byte) {
	if len(message) >= SMB2HeaderSize && len(signature) >= SignatureLength {
		copy(message[SignatureOffset:SignatureOffset+SignatureLength], signature[:SignatureLength])
	}
}

// SetSignedFlag sets the SMB2_FLAGS_SIGNED flag in the message header
func SetSignedFlag(message []byte) {
	if len(message) >= 20 {
		// Flags are at offset 16 (4 bytes, little-endian)
		flags := binary.LittleEndian.Uint32(message[16:20])
		flags |= SMB2_FLAGS_SIGNED
		binary.LittleEndian.PutUint32(message[16:20], flags)
	}
}

// IsMessageSigned checks if an SMB2 message has the signed flag set
func IsMessageSigned(message []byte) bool {
	if len(message) < 20 {
		return false
	}
	flags := binary.LittleEndian.Uint32(message[16:20])
	return flags&SMB2_FLAGS_SIGNED != 0
}

// =====================================================
// SMB 3.1.1 Pre-authentication Integrity Hash Functions
// =====================================================

// InitPreauthHash initializes the preauth integrity hash for SMB 3.1.1
// Per MS-SMB2 3.2.5.2, the initial hash is 64 zero bytes
func InitPreauthHash() []byte {
	return make([]byte, 64)
}

// UpdatePreauthHash updates the preauth integrity hash with a message
// Per MS-SMB2 3.2.5.2:
//   PreauthIntegrityHashValue = SHA-512(PreviousHash || MessageBytes)
// This is called after NEGOTIATE request/response and SESSION_SETUP request/response
func UpdatePreauthHash(currentHash []byte, message []byte) []byte {
	if len(currentHash) == 0 {
		currentHash = InitPreauthHash()
	}

	h := sha512.New()
	h.Write(currentHash)
	h.Write(message)
	newHash := h.Sum(nil)

	// Show first 16 bytes of message for debugging
	msgPreview := message
	if len(msgPreview) > 16 {
		msgPreview = msgPreview[:16]
	}
	log.Printf("[DEBUG] UpdatePreauthHash: prevHash=%x... msgLen=%d msgStart=%x newHash=%x...",
		currentHash[:16], len(message), msgPreview, newHash[:16])

	return newHash
}
