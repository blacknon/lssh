package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

func fileChecksum(filesystem FileSystem, path string) (string, error) {
	file, err := filesystem.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
