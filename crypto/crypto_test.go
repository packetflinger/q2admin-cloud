package crypto

import (
	"bytes"
	"fmt"
	"testing"
)

func TestRandomBytes(t *testing.T) {
	tests := []struct {
		desc   string
		length int
	}{
		{desc: "test1", length: 16},
		{desc: "test2", length: 128},
		{desc: "test3", length: 5000},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			data := RandomBytes(tc.length)
			if len(data) != tc.length {
				t.Error("got data length:", len(data), "want length:", tc.length)
			}
		})
	}
}

func TestLoadPrivateKey(t *testing.T) {
	tests := []struct {
		desc       string
		privatekey string
	}{
		{
			desc:       "test1",
			privatekey: ";kljasflkjafojwefoijwfjkslfsdf",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			privkey, err := LoadPrivateKey([]byte(tc.privatekey))
			if err != nil {
				t.Error(err)
			}
			err = privkey.Validate()
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestStreamRoundTrip(t *testing.T) {
	key := RandomBytes(AESKeyLength)
	send, err := NewStream(key, ToFrontend)
	if err != nil {
		t.Fatal(err)
	}
	recv, err := NewStream(key, ToFrontend)
	if err != nil {
		t.Fatal(err)
	}
	for _, msg := range []string{"hi there", "", "My hyperblaster is jammed!"} {
		ct, err := send.Seal([]byte(msg))
		if err != nil {
			t.Fatal(err)
		}
		if len(ct) != len(msg)+GCMTagLength {
			t.Errorf("ciphertext is %d bytes, want %d", len(ct), len(msg)+GCMTagLength)
		}
		got, err := recv.Open(ct)
		if err != nil {
			t.Fatalf("Open(%q): %v", msg, err)
		}
		if string(got) != msg {
			t.Errorf("got %q, want %q", got, msg)
		}
	}
}

// The same plaintext must never encrypt the same way twice, or an observer
// learns when messages repeat.
func TestStreamNoncesChange(t *testing.T) {
	s, _ := NewStream(RandomBytes(AESKeyLength), ToFrontend)
	a, _ := s.Seal([]byte("status"))
	b, _ := s.Seal([]byte("status"))
	if bytes.Equal(a, b) {
		t.Error("two frames with the same plaintext encrypted identically")
	}
}

// Anything that would have made CBC hand back garbage has to fail outright.
func TestStreamRejects(t *testing.T) {
	key := RandomBytes(AESKeyLength)
	seal := func(s *Stream, msg string) []byte {
		ct, err := s.Seal([]byte(msg))
		if err != nil {
			t.Fatal(err)
		}
		return ct
	}
	tests := []struct {
		desc string
		run  func() error
	}{
		{"wrong key", func() error {
			s, _ := NewStream(key, ToFrontend)
			r, _ := NewStream(RandomBytes(AESKeyLength), ToFrontend)
			_, err := r.Open(seal(s, "map q2dm1"))
			return err
		}},
		{"other direction", func() error {
			s, _ := NewStream(key, ToBackend)
			r, _ := NewStream(key, ToFrontend)
			_, err := r.Open(seal(s, "map q2dm1"))
			return err
		}},
		{"out of order", func() error {
			s, _ := NewStream(key, ToFrontend)
			r, _ := NewStream(key, ToFrontend)
			seal(s, "first")
			_, err := r.Open(seal(s, "second"))
			return err
		}},
		{"replayed", func() error {
			s, _ := NewStream(key, ToFrontend)
			r, _ := NewStream(key, ToFrontend)
			ct := seal(s, "kick 3")
			if _, err := r.Open(ct); err != nil {
				return nil // wrong failure, the check below reports it
			}
			_, err := r.Open(ct)
			return err
		}},
		{"tampered", func() error {
			s, _ := NewStream(key, ToFrontend)
			r, _ := NewStream(key, ToFrontend)
			ct := seal(s, "map q2dm1")
			ct[0] ^= 1
			_, err := r.Open(ct)
			return err
		}},
		{"too short", func() error {
			r, _ := NewStream(key, ToFrontend)
			_, err := r.Open(make([]byte, GCMTagLength-1))
			return err
		}},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.run() == nil {
				t.Error("frame opened, want an error")
			}
		})
	}
}

// q2admin's test_gcm_vectors (tests/test_cloud.c) checks its OpenSSL code
// produces these same bytes. If both pass, the two ends seal frames
// identically.
func TestStreamVectors(t *testing.T) {
	key := make([]byte, AESKeyLength)
	for i := range key {
		key[i] = byte(i*7 + 1)
	}
	tests := []struct {
		direction uint32
		want      []byte
	}{
		{ToBackend, []byte{
			0x0e, 0x50, 0xb9, 0xc6, 0xd4, 0x47, 0x63, 0x97, 0x35, 0xbc, 0xed,
			0xdc, 0xe5, 0x4f, 0x8f, 0x92, 0xdc, 0xb6, 0x78, 0xcf, 0xa4,
		}},
		{ToFrontend, []byte{
			0x16, 0x21, 0x99, 0xa0, 0x16, 0x5b, 0x30, 0xe8, 0x2d, 0x28, 0x7f,
			0x8f, 0x55, 0xbe, 0x06, 0x7d, 0xa6, 0xc6, 0x97, 0xf5, 0x9d,
		}},
	}
	for _, tc := range tests {
		s, err := NewStream(key, tc.direction)
		if err != nil {
			t.Fatal(err)
		}
		s.count = 5
		got, err := s.Seal([]byte("q2dm1"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, tc.want) {
			t.Errorf("direction %d: sealed = % x, want % x", tc.direction, got, tc.want)
		}
	}
}

// Pins the nonce layout, which q2admin has to build the same way.
func TestStreamNonceLayout(t *testing.T) {
	s := &Stream{direction: ToBackend, count: 0x0102030405060708}
	n, err := s.nonce()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{0, 0, 0, 1, 1, 2, 3, 4, 5, 6, 7, 8}
	if !bytes.Equal(n, want) {
		t.Errorf("nonce = % x, want % x", n, want)
	}
}

/*
func TestMD5Hash(t *testing.T) {
	tests := []struct {
		desc       string
		plaintext  string
		wantcipher []uint8
	}{
		{
			desc:      "test1",
			plaintext: "blah blah blah",
			wantcipher: []byte{
				0x65, 0x35, 0x38, 0x64, 0x36, 0x36, 0x36, 0x64,
				0x66, 0x30, 0x64, 0x36, 0x63, 0x63, 0x36, 0x65,
				0x38, 0x37, 0x61, 0x37, 0x65, 0x34, 0x38, 0x34,
				0x34, 0x30, 0x64, 0x36, 0x33, 0x37, 0x65, 0x39,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got := MD5Hash(tc.plaintext)
			want := string(tc.wantcipher)
			if got != want {
				t.Error("got:", got, "want:", want)
			}
		})
	}
}
*/

func TestSHA256Hash(t *testing.T) {
	tests := []struct {
		desc       string
		plaintext  string
		wantcipher string
	}{
		{
			desc:       "test1",
			plaintext:  "blah blah blah",
			wantcipher: "a74f733635a19aefb1f73e5947cef59cd7440c6952ef0f03d09d974274cbd6df",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := MessageDigest([]byte(tc.plaintext))
			if err != nil {
				t.Error(err)
			}
			gottxt := fmt.Sprintf("%x", got)
			if gottxt != tc.wantcipher {
				t.Errorf("\ngot:  %s\nwant: %s\n", gottxt, tc.wantcipher)
			}
		})
	}
}

func TestGenerateKeyPairRoundTrip(t *testing.T) {
	priv, pub, err := GenerateKeyPair(2048)
	if err != nil {
		t.Fatal(err)
	}
	privKey, err := LoadPrivateKey(priv)
	if err != nil {
		t.Fatalf("LoadPrivateKey: %v", err)
	}
	pubKey, err := LoadPublicKey(pub)
	if err != nil {
		t.Fatalf("LoadPublicKey: %v", err)
	}
	if !privKey.PublicKey.Equal(pubKey) {
		t.Error("public key doesn't match the private key")
	}
	if _, _, err := GenerateKeyPair(1024); err == nil {
		t.Error("1024-bit keys should be refused")
	}
}

func TestLoadPublicKeyRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "not a key", "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n"} {
		if _, err := LoadPublicKey([]byte(in)); err == nil {
			t.Errorf("LoadPublicKey(%q) succeeded", in)
		}
	}
}
