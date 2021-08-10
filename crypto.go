package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
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
