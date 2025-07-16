package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log"

	"golang.org/x/crypto/nacl/box"
)

// Authorize is a stub for agent registration authorization.
func Authorize(_ string, _ string) bool {
	// TODO: implement real authorization logic
	return true
}

// VerifyToken checks if the provided token matches the expected token for the sender.
func VerifyToken(getToken func(string) string, sender, token string) bool {
	expected := getToken(sender)
	return token == expected
}

// VerifyHMAC checks if the provided signature matches the HMAC of the message using the sender's token (if present in DB).
func VerifyHMAC(getToken func(string) string, sender, message, signature string) bool {
	token := getToken(sender)
	if token == "" {
		log.Println("[DEBUG] HMAC token not defined")
		return false
	}
	mac := hmac.New(sha256.New, []byte(token))
	mac.Write([]byte(sender + message))
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	res := hmac.Equal([]byte(signature), []byte(expectedMAC))
	return res
}

// ComputeHMAC computes the HMAC signature for a message using the given key.
func ComputeHMAC(sender, message, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(sender + message))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateKeyPair generates a new ephemeral public/private keypair for the agent (X25519/NaCl box).
func GenerateKeyPair() (publicKey, privateKey *[32]byte, err error) {
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	return pub, priv, nil
}

// EncryptWithPublicKey encrypts the given message (HMAC key) with the recipient's public key using NaCl box.
func EncryptWithPublicKey(message []byte, recipientPub *[32]byte) (string, error) {
	// Generate ephemeral key for sender
	ephPub, ephPriv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	// Nonce must be unique per message
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	// Encrypt the message
	encrypted := box.Seal(ephPub[:], message, &nonce, recipientPub, ephPriv)
	// Prepend ephemeral public key to the ciphertext
	// Format: [ephemeral pub (32 bytes)] + [ciphertext]
	full := append(nonce[:], encrypted...)
	return base64.StdEncoding.EncodeToString(full), nil
}

// DecryptWithPrivateKey decrypts the message using the agent's private key.
func DecryptWithPrivateKey(ciphertextB64 string, priv *[32]byte) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	if len(data) < 24+32 {
		return nil, err
	}
	var nonce [24]byte
	copy(nonce[:], data[:24])
	// The next 32 bytes are the ephemeral public key
	var ephPub [32]byte
	copy(ephPub[:], data[24:56])
	// The rest is the ciphertext
	ciphertext := data[56:]
	// Decrypt
	decrypted, ok := box.Open(nil, ciphertext, &nonce, &ephPub, priv)
	if !ok {
		return nil, err
	}
	return decrypted, nil
}

// GenerateRandomToken generates a random base64-encoded token of n bytes.
func GenerateRandomToken(n int) string {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "fallbacktoken"
	}
	return base64.StdEncoding.EncodeToString(b)
}
