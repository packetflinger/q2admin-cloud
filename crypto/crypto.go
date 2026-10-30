// The crypto package holds almost all code needed for any
// cryptographic function we require.
// These include:
//   - key generation
//   - asymmetric (en|de)crypt
//   - symmetric (en|de)crypt
//   - signing/verifying
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
)

const (
	AESKeyLength   = 16  // 128 bit
	GCMNonceLength = 12  // 96 bit, the size GCM is designed for
	GCMTagLength   = 16  // 128 bit authentication tag on every frame
	RSAKeyLength   = 256 // 2048 bits
	DigestLength   = 32  // 256 bits
)

// Directions, used as the fixed part of a GCM nonce. Both directions of a
// connection share one key, so this is what keeps a nonce used by one side
// from ever being used by the other. q2admin has the same values.
const (
	ToBackend  uint32 = 1 // frames sent by q2admin
	ToFrontend uint32 = 2 // frames sent by us
)

// Get a hash of an input byte slice
// Currently using SHA256, if that changes, update the DigestLength constant above!
func MessageDigest(input []byte) ([]byte, error) {
	if len(input) == 0 {
		return []byte{}, fmt.Errorf("hashing error: empty input")
	}
	hash := sha256.New()
	_, err := hash.Write(input)
	if err != nil {
		return []byte{}, fmt.Errorf("error calculating sha256 hash: %v", err)
	}
	checksum := hash.Sum(nil)
	return checksum, nil
}

// Get an MD5 hash of an input string. Just used for
// change comparisons.
func MD5Hash(input string) string {
	if input == "" {
		return ""
	}
	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

// GenerateKeyPair makes an RSA key pair and returns it PEM-encoded: the
// private key as PKCS#8 ("PRIVATE KEY", what openssl genpkey writes) and the
// public key as PKIX ("PUBLIC KEY").
func GenerateKeyPair(bitlength int) (private, public []byte, err error) {
	if bitlength < 2048 {
		return nil, nil, fmt.Errorf("key length %d is too short, use at least 2048", bitlength)
	}
	key, err := rsa.GenerateKey(rand.Reader, bitlength)
	if err != nil {
		return nil, nil, fmt.Errorf("generating RSA key: %v", err)
	}
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding private key: %v", err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding public key: %v", err)
	}
	private = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	public = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	return private, public, nil
}

// Read an RSA private key into memory from the filesystem
func LoadPrivateKey(keybytes []byte) (*rsa.PrivateKey, error) {
	var err error
	privPem, _ := pem.Decode(keybytes)
	if privPem == nil || privPem.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("not a private key file")
	}
	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPem.Bytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPem.Bytes); err != nil { // note this returns type `interface{}`
			return nil, fmt.Errorf("unable to parse private key file")
		}
	}
	if privateKey, ok := parsedKey.(*rsa.PrivateKey); ok {
		return privateKey, nil
	}
	return nil, fmt.Errorf("something went wrong parsing private key")
}

// Read an RSA public key from the filesystem and get it ready to use
func LoadPublicKey(pub []byte) (*rsa.PublicKey, error) {
	pubPem, _ := pem.Decode(pub)
	if pubPem == nil {
		return nil, errors.New("no PEM data found")
	}
	if pubPem.Type == "PUBLIC KEY" {
		public, err := x509.ParsePKIXPublicKey(pubPem.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKIX key: %v", err)
		}
		rsaKey, ok := public.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return rsaKey, nil
	}
	if pubPem.Type == "RSA PUBLIC KEY" {
		public, err := x509.ParsePKCS1PublicKey(pubPem.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parsing PKCS1 key: %v", err)
		}
		return public, nil
	}
	return nil, errors.New("not a public key file")
}

// Decrypt a block of ciphertext using our private key.
//
// This is used as part of authenticating clients. During the connection
// handshake, the client will generate a random block of data and encrypt it
// using the server's public key. The decrypted data is sent back to the client
// to prove the server has the matching private key, authenticating the server.
func PrivateDecrypt(key *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	if key == nil {
		return []byte{}, fmt.Errorf("error decrypting: null key")
	}
	plaintext, err := key.Decrypt(nil, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("asymmetric decrypt failed: %v", err)
	}
	return plaintext, nil
}

// Encrypt a block of plaintext using the client's public key.
//
// This is used as part of authentication. In order to authenticate the client,
// the server will generate a random block of data, asymmetrically encrypt it
// using the client's public key and send it over. Only the client's private
// key will be able to decrypt it, so the client will decrypt and send back a
// hash of the data. If the hashes match, the client is successfully
// authenticated.
func PublicEncrypt(key *rsa.PublicKey, plaintext []byte) ([]byte, error) {
	if key == nil {
		return []byte{}, fmt.Errorf("error encrypting: null key")
	}
	encryptedBytes, err := rsa.EncryptPKCS1v15(rand.Reader, key, plaintext)
	if err != nil {
		return nil, fmt.Errorf("asymmetric encrypt failed: %v", err)
	}
	return encryptedBytes, nil
}

// Get a byte slice of random data (for generating keys)
func RandomBytes(length int) []byte {
	if length < 1 {
		return []byte{}
	}
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil
	}

	return b
}

// Hash the plaintext, encrypt the resulting digest with our private key.
// Only our public key can decrypt, proving it's really us
func Sign(key *rsa.PrivateKey, plaintext []byte) []byte {
	if key == nil {
		return []byte{}
	}
	hash := sha256.New()
	_, _ = hash.Write(plaintext)

	checksum := hash.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, 5, checksum)
	if err != nil {
		fmt.Println(err)
	}

	return signature
}

// Stream encrypts or decrypts the frames going one way over a connection,
// using AES-128-GCM. Every frame is sealed with its own nonce and carries a
// 16-byte tag, so a frame that was altered, replayed, reordered or decrypted
// with the wrong key fails to open instead of turning into garbage.
//
// Nonces follow NIST SP 800-38D section 8.2.1 (the "deterministic
// construction"), the same scheme TLS 1.3 and SSH use: 4 bytes saying which
// direction the frame goes, then a 64-bit count of the frames already sent
// that way with this key, both big-endian. Nothing about the nonce is sent;
// TCP delivers frames in order, so both ends count them. A nonce can never
// repeat under one key, which is the one thing GCM needs: the direction
// keeps the two sides apart, and the count only goes up. A new key (the
// handshake, or a rotation) starts the count again at zero.
//
// A Stream isn't safe for concurrent use.
type Stream struct {
	aead      cipher.AEAD
	direction uint32
	count     uint64
}

// NewStream returns a Stream for frames going in direction (ToBackend or
// ToFrontend), encrypted with key.
func NewStream(key []byte, direction uint32) (*Stream, error) {
	if len(key) != AESKeyLength {
		return nil, fmt.Errorf("AES key is %d bytes, want %d", len(key), AESKeyLength)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Stream{aead: aead, direction: direction}, nil
}

// nonce returns the nonce for the next frame, or an error if the count has
// run out (it never will in practice: 2^64 frames).
func (s *Stream) nonce() ([]byte, error) {
	if s.count == math.MaxUint64 {
		return nil, errors.New("GCM frame count exhausted, a new key is needed")
	}
	n := make([]byte, GCMNonceLength)
	binary.BigEndian.PutUint32(n[0:4], s.direction)
	binary.BigEndian.PutUint64(n[4:], s.count)
	return n, nil
}

// Seal encrypts one frame's payload and returns the ciphertext with the tag
// on the end (GCMTagLength bytes longer than plaintext).
func (s *Stream) Seal(plaintext []byte) ([]byte, error) {
	n, err := s.nonce()
	if err != nil {
		return nil, err
	}
	s.count++
	return s.aead.Seal(nil, n, plaintext, nil), nil
}

// Open decrypts one frame's payload. Any error means the frame can't be
// trusted, and since the count is now out of step with the sender's, the
// connection can't continue.
func (s *Stream) Open(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < GCMTagLength {
		return nil, fmt.Errorf("encrypted frame too short (%d bytes)", len(ciphertext))
	}
	n, err := s.nonce()
	if err != nil {
		return nil, err
	}
	plaintext, err := s.aead.Open(nil, n, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("frame %d failed authentication", s.count)
	}
	s.count++
	return plaintext, nil
}

// Use a public key to decrypt a signature and compare it to hash of the content
func VerifySignature(key *rsa.PublicKey, plaintext []byte, sig []byte) bool {
	if key == nil {
		return false
	}
	hash := sha256.New()
	temp := plaintext
	_, _ = hash.Write(temp)
	checksum := hash.Sum(nil)

	err := rsa.VerifyPKCS1v15(key, 5, checksum, sig)

	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
