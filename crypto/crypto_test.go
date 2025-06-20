package crypto

import (
	"fmt"
	"reflect"
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
			privatekey: "private.pem",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			privkey, err := LoadPrivateKey(tc.privatekey)
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

func TestSymmetricEncryption(t *testing.T) {
	key := RandomBytes(16)
	iv := RandomBytes(16)

	tests := []struct {
		desc      string
		plaintext string
	}{
		{desc: "test1", plaintext: "hi there"},
		{desc: "test2", plaintext: "armor 20"},
		{desc: "test3", plaintext: "My hyperblaster is jammed!"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			cipher := SymmetricEncrypt(key, iv, []byte(tc.plaintext))
			plain, _ := SymmetricDecrypt(key, iv, cipher)
			if !reflect.DeepEqual(plain, []byte(tc.plaintext)) {
				t.Error("got:", plain, "want:", []byte(tc.plaintext))
			}
		})
	}
}

func TestSymmetricDecrypt(t *testing.T) {
	tests := []struct {
		name   string
		cipher string
		key    string
		iv     string
		want   string
	}{
		{
			name:   "test1",
			cipher: "d3 91 59 06 33 0b fa 8e  c3 76 c9 dc 07 3c a8 95",
			key:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

		})
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
