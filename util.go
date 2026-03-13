//go:build linux

package main

import (
	"crypto/rand"
	"encoding/hex"
)

func generateId() (string, error) {
	const prefix = "vm-"
	const maxLen = 15

	suffixLen := maxLen - len(prefix)
	byteLen := suffixLen / 2 // 2 hex chars per byte

	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	id := prefix + hex.EncodeToString(b)

	return id, nil
}
