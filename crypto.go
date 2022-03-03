package main

import (
	"crypto/aes"
	"crypto/cipher"
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

/**
 * Generate a private/public key pair of a certain bit length
 */
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

func LoadPrivateKey(keyfile string) (*rsa.PrivateKey, error) {
	priv, err := os.ReadFile(keyfile)
	if err != nil {
		panic(err)
	}
	privPem, _ := pem.Decode(priv)

	if privPem.Type != "RSA PRIVATE KEY" {
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

func PrivateDecrypt(key *rsa.PrivateKey, ciphertext []byte) []byte {
	plaintext, err := key.Decrypt(
		nil,
		ciphertext,
		&rsa.OAEPOptions{Hash: 5}) // sha256 (https://pkg.go.dev/crypto#DecrypterOpts)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return plaintext
}

func PublicEncrypt(key *rsa.PublicKey, plaintext []byte) []byte {
	/*encryptedBytes, err := rsa.EncryptOAEP(
	sha256.New(),
	rand.Reader,
	key,
	plaintext,
	nil)*/

	encryptedBytes, err := rsa.EncryptPKCS1v15(rand.Reader, key, plaintext)

	if err != nil {
		panic(err)
	}

	return encryptedBytes
}

func RandomBytes(length int) []byte {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		return nil
	}

	return b
}

/**
 * Hash the plaintext, encrypt the resulting digest with our private key.
 * Only our public key can decrypt, proving it's really us
 */
func Sign(key *rsa.PrivateKey, plaintext []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write(plaintext)

	checksum := hash.Sum(nil)

	//signature, _ := rsa.SignPSS(rand.Reader, key, 5, checksum, nil)
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, 5, checksum)
	if err != nil {
		fmt.Println(err)
	}

	//fmt.Printf("Signature:\n\n%s\n", hex.Dump(signature))
	return signature
}

func SymmetricDecrypt(key []byte, nonce []byte, ciphertext []byte) ([]byte, int) {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println("newcipher:", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		fmt.Println("newgcm:", err)
	}
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		fmt.Println("open:", err)
	}
	//fmt.Printf("cipher:\n%s\n", hex.Dump(ciphertext))
	//fmt.Printf("plain:\n%s\n", hex.Dump(plaintext))
	return plaintext, len(plaintext)
}

func SymmetricEncrypt(key []byte, nonce []byte, plaintext []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println(err)
	}
	gcm, _ := cipher.NewGCM(block)

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	fmt.Printf("cipher:\n%s\n", hex.Dump(ciphertext))
	return ciphertext
}

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

func DigestSHA256(input []byte) []byte {
	hash := sha256.New()
	_, _ = hash.Write(input)
	checksum := hash.Sum(nil)
	return checksum
}
