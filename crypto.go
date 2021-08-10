package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/sha256"
    "crypto/x509"
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
    privPem, _ := pem.Decode(priv)

	if privPem.Type != "RSA PRIVATE KEY" {
		return nil, errors.New("Not a private key file")
	}

    var parsedKey interface{}
	if parsedKey, err = x509.ParsePKCS1PrivateKey(privPem.Bytes); err != nil {
		if parsedKey, err = x509.ParsePKCS8PrivateKey(privPem.Bytes); err != nil { // note this returns type `interface{}`
            return nil, errors.New("Unable to parse private key file")
		}
	}

	var privateKey *rsa.PrivateKey
	var ok bool
	privateKey, ok = parsedKey.(*rsa.PrivateKey)

    if ok {
        return privateKey, nil
    }

    return nil, errors.New("Something went wrong")
}

func PrivateDecrypt(key *rsa.PrivateKey, ciphertext []byte) []byte {
    plaintext, err := key.Decrypt(
        nil,
        ciphertext,
        &rsa.OAEPOptions{Hash: 5})  // sha256 (https://pkg.go.dev/crypto#DecrypterOpts)
    if err != nil {
        fmt.Println(err)
        return nil
    }

    return plaintext
}

func PublicEncrypt(key *rsa.PublicKey, plaintext []byte) []byte {
    encryptedBytes, err := rsa.EncryptOAEP(
    	sha256.New(),
    	rand.Reader,
    	key,
    	plaintext,
    	nil)

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
