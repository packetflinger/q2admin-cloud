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
	"os"
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
	hash := md5.Sum([]byte(input))
	return hex.EncodeToString(hash[:])
}

// Generate a private/public key pair of a certain bit length
func GenerateKeys(bitlength int) bool {
	// make the actual keys
	privatekey, err := rsa.GenerateKey(rand.Reader, bitlength)
	if err != nil {
		fmt.Printf("Cannot generate RSA key\n")
		return false
	}
	publickey := &privatekey.PublicKey

	// dump to disk
	var privBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	}
	privatePem, err := os.Create("private.pem")
	if err != nil {
		fmt.Printf("error when create private.pem: %s \n", err)
		return false
	}
	err = pem.Encode(privatePem, privateKeyBlock)
	if err != nil {
		fmt.Printf("error when encode private pem: %s \n", err)
		return false
	}

	// dump public key to disk
	pubBytes, err := x509.MarshalPKIXPublicKey(publickey)
	if err != nil {
		fmt.Printf("error when dumping publickey: %s \n", err)
		return false
	}
	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}
	publicPem, err := os.Create("public.pem")
	if err != nil {
		fmt.Printf("error when create public.pem: %s \n", err)
		return false
	}
	err = pem.Encode(publicPem, publicKeyBlock)
	if err != nil {
		fmt.Printf("error when encode public pem: %s \n", err)
		return false
	}

	return true
}

// Read an RSA private key into memory from the filesystem
func LoadPrivateKey(keyfile string) (*rsa.PrivateKey, error) {
	priv, err := os.ReadFile(keyfile)
	if err != nil {
		panic(err)
	}
	privPem, _ := pem.Decode(priv)

	if privPem.Type != "PRIVATE KEY" {
		return nil, errors.New("not a private key file")
	}

	var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPem.Bytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPem.Bytes); err != nil { // note this returns type `interface{}`
			return nil, errors.New("unable to parse private key file")
		}
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedKey.(*rsa.PrivateKey)

	if ok {
		return privateKey, nil
	}

	return nil, errors.New("something went wrong")
}

// Read an RSA public key from the filesystem and get
// it ready to use
func LoadPublicKey(keyfile string) (*rsa.PublicKey, error) {
	pub, err := os.ReadFile(keyfile)
	if err != nil {
		return nil, err
	}

	pubPem, _ := pem.Decode(pub)

	if pubPem.Type == "PUBLIC KEY" {
		public, _ := x509.ParsePKIXPublicKey(pubPem.Bytes)
		return public.(*rsa.PublicKey), nil
	}

	if pubPem.Type == "RSA PUBLIC KEY" {
		public, _ := x509.ParsePKCS1PublicKey(pubPem.Bytes)
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
	encryptedBytes, err := rsa.EncryptPKCS1v15(rand.Reader, key, plaintext)
	if err != nil {
		return nil, fmt.Errorf("asymmetric encrypt failed: %v", err)
	}
	return encryptedBytes, nil
}

// Get a byte slice of random data (for generating keys)
func RandomBytes(length int) []byte {
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
	hash := sha256.New()
	_, _ = hash.Write(plaintext)

	checksum := hash.Sum(nil)

	signature, err := rsa.SignPKCS1v15(rand.Reader, key, 5, checksum)
	if err != nil {
		fmt.Println(err)
	}

	return signature
}

// Decrypt incoming messages using AES
func SymmetricDecrypt(key []byte, nonce []byte, ciphertext []byte) ([]byte, int) {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println("newcipher:", err)

	}

	plaintext := make([]byte, len(ciphertext))
	cbc := cipher.NewCBCDecrypter(block, nonce)
	cbc.CryptBlocks(plaintext, ciphertext)

	// the last byte
	padding := int(plaintext[len(plaintext)-1])

	// padding bit should never exceed message length,
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
