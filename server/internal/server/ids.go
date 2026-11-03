package server

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
)

// roomAlphabet leaves out look-alike characters (0/O, 1/I).
const roomAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newRoomCode() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	out := make([]byte, 4)
	for i, x := range b {
		out[i] = roomAlphabet[int(x)%len(roomAlphabet)]
	}
	return string(out)
}

func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randomSeed() uint64 {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return binary.LittleEndian.Uint64(b)
}
