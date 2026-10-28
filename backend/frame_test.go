package backend

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"testing/iotest"

	"github.com/packetflinger/q2admind/crypto"
	"github.com/packetflinger/q2admind/frontend"
)

func TestMakeFrame(t *testing.T) {
	// what q2admin's test_cloud.c sees for CMD_PING followed by "hi"
	want := []byte{4, 0, 0, 0, CMDPing, 'h', 'i', 0}
	got := MakeFrame([]byte{CMDPing, 'h', 'i', 0})
	if !bytes.Equal(got, want) {
		t.Errorf("MakeFrame() = %v, want %v", got, want)
	}
}

func TestReadFrame(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    [][]byte
		wantErr error // after the frames in want
	}{
		{
			name:    "one frame",
			input:   MakeFrame([]byte{SCMDPong}),
			want:    [][]byte{{SCMDPong}},
			wantErr: io.EOF,
		},
		{
			name:    "two frames back to back",
			input:   append(MakeFrame([]byte{1, 2, 3}), MakeFrame([]byte{4, 5})...),
			want:    [][]byte{{1, 2, 3}, {4, 5}},
			wantErr: io.EOF,
		},
		{
			name:    "empty frame",
			input:   MakeFrame(nil),
			want:    [][]byte{{}},
			wantErr: io.EOF,
		},
		{
			name:    "closed partway through the payload",
			input:   MakeFrame([]byte{1, 2, 3})[:5],
			wantErr: io.ErrUnexpectedEOF,
		},
		{
			name:    "closed partway through the header",
			input:   []byte{3, 0},
			wantErr: io.ErrUnexpectedEOF,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// one byte per Read, the worst a TCP stream can do
			r := iotest.OneByteReader(bytes.NewReader(tc.input))
			for i, want := range tc.want {
				got, err := ReadFrame(r)
				if err != nil {
					t.Fatalf("frame %d: ReadFrame() error = %v", i, err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("frame %d: ReadFrame() = %v, want %v", i, got, want)
				}
			}
			if _, err := ReadFrame(r); !errors.Is(err, tc.wantErr) {
				t.Errorf("ReadFrame() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestReadFrameTooLong(t *testing.T) {
	header := []byte{0xff, 0xff, 0xff, 0x7f}
	if _, err := ReadFrame(bytes.NewReader(header)); err == nil {
		t.Error("ReadFrame() with an impossible length returned no error")
	}
	ok := MakeFrame(make([]byte, FrameMaxLength))
	if _, err := ReadFrame(bytes.NewReader(ok)); err != nil {
		t.Errorf("ReadFrame() of a FrameMaxLength frame: %v", err)
	}
}

// pipeFrontend makes a frontend whose connection is one end of a pipe, and
// returns the other end to read what it sends.
func pipeFrontend(t *testing.T) (*frontend.Frontend, net.Conn) {
	t.Helper()
	ours, theirs := net.Pipe()
	t.Cleanup(func() {
		ours.Close()
		theirs.Close()
	})
	return &frontend.Frontend{Name: "test", Server: &Backend{}, Connection: &ours}, theirs
}

// sendAndRead runs SendMessages and reads the frame it sends.
func sendAndRead(t *testing.T, fe *frontend.Frontend, peer net.Conn) []byte {
	t.Helper()
	got := make(chan []byte)
	go func() {
		payload, err := ReadFrame(peer)
		if err != nil {
			t.Errorf("ReadFrame() error = %v", err)
		}
		got <- payload
	}()
	SendMessages(fe)
	return <-got
}

func TestSendMessagesFrames(t *testing.T) {
	fe, peer := pipeFrontend(t)
	fe.MessageOut.WriteByte(SCMDPong)
	fe.MessageOut.WriteString("hello")

	got := sendAndRead(t, fe, peer)
	want := []byte{SCMDPong, 'h', 'e', 'l', 'l', 'o', 0}
	if !bytes.Equal(got, want) {
		t.Errorf("payload = %v, want %v", got, want)
	}
	if fe.MessageOut.Size() != 0 {
		t.Errorf("MessageOut not emptied, %d bytes left", fe.MessageOut.Size())
	}
}

func TestSendMessagesEncryptedFrames(t *testing.T) {
	fe, peer := pipeFrontend(t)
	fe.Trusted = true
	fe.Encrypted = true
	fe.SymmetricKey = crypto.RandomBytes(crypto.AESBlockLength)
	fe.InitVector = crypto.RandomBytes(crypto.AESIVLength)
	iv := fe.InitVector
	fe.MessageOut.WriteByte(SCMDPong)

	got := sendAndRead(t, fe, peer)
	if len(got)%crypto.AESBlockLength != 0 {
		t.Fatalf("encrypted payload is %d bytes, not whole blocks", len(got))
	}
	plain, size := crypto.SymmetricDecrypt(fe.SymmetricKey, iv, got)
	if size != 1 || plain[0] != SCMDPong {
		t.Errorf("decrypted payload = %v, want [%d]", plain[:size], SCMDPong)
	}
	// the next IV is the first ciphertext block, as q2admin expects
	if !bytes.Equal(fe.InitVector, got[:crypto.AESIVLength]) {
		t.Error("InitVector wasn't moved on to the first ciphertext block")
	}
	if !bytes.Equal(fe.PreviousIV, iv) {
		t.Error("PreviousIV isn't the IV the frame was encrypted with")
	}
}

func TestSendMessagesDropsOversizedFrame(t *testing.T) {
	fe, _ := pipeFrontend(t)
	fe.MessageOut.WriteData(make([]byte, FrameMaxLength+1))
	SendMessages(fe) // would block on the pipe if it tried to write
	if fe.MessageOut.Size() != 0 {
		t.Errorf("MessageOut not emptied, %d bytes left", fe.MessageOut.Size())
	}
}
