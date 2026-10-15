// The crypto package holds almost all code needed for any
// cryptographic function we require.
// These include:
//   - key generation
//   - asymmetric (en|de)crypt
//   - symmetric (en|de)crypt
//   - signing/verifying
package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
)

const (
	AESBlockLength = 16  // 128 bit
	AESIVLength    = 16  // 128 bit
	RSAKeyLength   = 256 // 2048 bits
	DigestLength   = 32  // 256 bits
)

type EncryptionKey struct {
	Key        []byte // 16 bytes (128 bit)
	InitVector []byte // 16 bytes
}

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

// Manually add padding to a slice to get it to a specific
// block size. The number of bytes required is the byte used
// for the actual padding
func PKCS5Padding(input []byte, blockSize int) []byte {
	padding := (blockSize - len(input)%blockSize)
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(input, padtext...)
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

// Decrypt incoming messages using AES in CBC mode. Any errors encountered here
// are considered fatal, so errors are not returned.
func SymmetricDecrypt(key []byte, nonce []byte, ciphertext []byte) ([]byte, int) {
	if len(key) != len(nonce) {
		log.Panicf("crypto error: key and IV different lengths - %d, %d", len(key), len(nonce))
	}
	if len(ciphertext)%len(key) != 0 {
		log.Panicf("crypto error: key must match block length - %d, %d", len(key), len(ciphertext))
	}
	if len(ciphertext) == 0 {
		return []byte{}, 0
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		log.Panicln("error creating new cipher:", err)
	}

	plaintext := make([]byte, len(ciphertext))
	cbc := cipher.NewCBCDecrypter(block, nonce)
	cbc.CryptBlocks(plaintext, ciphertext)

	// The last byte value is the number of bytes padded.
	padding := int(plaintext[len(plaintext)-1])

	// Padding bit should never exceed message length, that would spill over
	// into a new block.
	if padding > len(plaintext) {
		return []byte{}, 0
	}
	unpadded := plaintext[:len(plaintext)-padding]
	return unpadded, len(unpadded)
}

// Encrypt outgoing messages using AES
func SymmetricEncrypt(key []byte, nonce []byte, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println(err)
	}

	plaintext = PKCS5Padding(plaintext, AESBlockLength)
	ciphertext := make([]byte, len(plaintext))
	cbc := cipher.NewCBCEncrypter(block, nonce)
	cbc.CryptBlocks(ciphertext, plaintext)

	return ciphertext
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
