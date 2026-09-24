package helper

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost = 12 — per Modul §3. Spec requires ≥10. 12 is 2^12 iterations.
// For reference: cost 10 ~60ms, 12 ~250ms, 14 ~1s on modern CPU.
// Chosen: 12 balances brute-force resistance vs login latency (Tugas Mandiri §1 must state reason).
const bcryptCost = 12

// dummyHash is a valid bcrypt hash used when username not found.
// Ensures timing of "user not found" ≈ "wrong password" → prevents user enumeration & timing attack.
var dummyHash = []byte("$2a$12$abcdefghijklmnopqrstuuLKa3Bt1TCmU/6zvhZ8x4nq1yBiuGvS")

// HashPassword hashes plain with bcrypt (salt auto-generated & embedded).
func HashPassword(plain string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// VerifyPassword compares plain against bcrypt hash.
func VerifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// VerifyDummyPassword burns similar time as VerifyPassword when user not found.
func VerifyDummyPassword(plain string) {
	_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(plain))
}

// RandomToken generates cryptographically secure random hex string (numBytes raw → 2*numBytes hex chars).
// Uses crypto/rand, NOT math/rand.
func RandomToken(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// SHA256Hex hashes value with SHA-256 and returns hex. Used for refresh_token storage.
func SHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
