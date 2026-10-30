package backend

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
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

// encryptedFrontend returns a trusted, encrypted frontend on a pipe, and the
// stream the frontend's end would open our frames with.
func encryptedFrontend(t *testing.T) (*frontend.Frontend, net.Conn, *crypto.Stream) {
	t.Helper()
	fe, peer := pipeFrontend(t)
	fe.Trusted = true
	fe.Encrypted = true
	key := crypto.RandomBytes(crypto.AESKeyLength)
	var err error
	if fe.SendStream, err = crypto.NewStream(key, crypto.ToFrontend); err != nil {
		t.Fatal(err)
	}
	if fe.RecvStream, err = crypto.NewStream(key, crypto.ToBackend); err != nil {
		t.Fatal(err)
	}
	theirs, err := crypto.NewStream(key, crypto.ToFrontend)
	if err != nil {
		t.Fatal(err)
	}
	return fe, peer, theirs
}

func TestSendMessagesEncryptedFrames(t *testing.T) {
	fe, peer, theirs := encryptedFrontend(t)
	for i := 0; i < 3; i++ {
		fe.MessageOut.WriteByte(SCMDPong)
		got := sendAndRead(t, fe, peer)
		if len(got) != 1+crypto.GCMTagLength {
			t.Fatalf("frame %d: encrypted payload is %d bytes, want %d", i, len(got), 1+crypto.GCMTagLength)
		}
		plain, err := theirs.Open(got)
		if err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
		if !bytes.Equal(plain, []byte{SCMDPong}) {
			t.Errorf("frame %d: decrypted payload = %v, want [%d]", i, plain, SCMDPong)
		}
	}
}

// Frames are sent from several goroutines at once. The frontend works out
// each frame's nonce by counting, so they have to arrive in the order they
// were sealed or it can't open them.
func TestSendMessagesConcurrentNonceOrder(t *testing.T) {
	fe, peer, theirs := encryptedFrontend(t)
	const senders, each = 8, 25
	done := make(chan error)
	go func() {
		for i := 0; i < senders*each; i++ {
			payload, err := ReadFrame(peer)
			if err != nil {
				done <- err
				return
			}
			if _, err := theirs.Open(payload); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	var wg sync.WaitGroup
	for g := 0; g < senders; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				fe.Mu.Lock()
				fe.MessageOut.WriteByte(SCMDPong)
				SendMessages(fe)
				fe.Mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// After a rotation, our frames use the new key from the one after the
// SCMDKey on, and theirs only once they've acked it.
func TestRotateKeys(t *testing.T) {
	fe, peer, theirs := encryptedFrontend(t)
	oldRecv := fe.RecvStream

	got := make(chan []byte)
	go func() {
		payload, err := ReadFrame(peer)
		if err != nil {
			t.Errorf("ReadFrame() error = %v", err)
		}
		got <- payload
	}()
	RotateKeys(fe)
	plain, err := theirs.Open(<-got)
	if err != nil {
		t.Fatalf("SCMDKey frame: %v", err)
	}
	if len(plain) != 1+crypto.AESKeyLength || plain[0] != SCMDKey {
		t.Fatalf("SCMDKey frame = %v", plain)
	}
	newKey := plain[1:]
	if fe.RecvStream != oldRecv || fe.NextRecv == nil {
		t.Fatal("receiving side switched before the ack")
	}

	// a second rotation before the ack would leave the ends unsure which
	// key is which, so it waits
	fe.MessageOut.WriteByte(SCMDPong)
	RotateKeys(fe)
	theirsNew, _ := crypto.NewStream(newKey, crypto.ToFrontend)
	plain, err = theirsNew.Open(sendAndRead(t, fe, peer))
	if err != nil {
		t.Fatalf("frame after SCMDKey: %v", err)
	}
	if !bytes.Equal(plain, []byte{SCMDPong}) {
		t.Errorf("frame after SCMDKey = %v, want only a pong (no second SCMDKey)", plain)
	}

	if !ParseKeyAck(fe) {
		t.Fatal("ParseKeyAck() = false with a rotation waiting")
	}
	if fe.RecvStream == oldRecv || fe.NextRecv != nil {
		t.Fatal("receiving side didn't switch on the ack")
	}
	frontSend, _ := crypto.NewStream(newKey, crypto.ToBackend)
	sealed, _ := frontSend.Seal([]byte{CMDPing})
	if _, err := fe.RecvStream.Open(sealed); err != nil {
		t.Errorf("frame under the new key after the ack: %v", err)
	}
	if ParseKeyAck(fe) {
		t.Error("ParseKeyAck() = true with no rotation waiting")
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
