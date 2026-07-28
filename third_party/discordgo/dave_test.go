package discordgo

import "testing"

func TestActivatePreparedTransitionActivatesSenderWhenKeyIsPrepared(t *testing.T) {
	d := &DAVESession{
		senderKey:           []byte{1, 2, 3},
		frameCipher:         testAEAD{},
		hasPendingKey:       true,
		exporterSecret:      []byte{9, 9, 9},
		pendingTransitionID: 0,
		pendingVersion:      1,
	}

	// Cipher presence alone means we can encrypt (WaitForDAVEReady / opusSender).
	if !d.CanEncrypt() {
		t.Fatal("expected frameCipher to make CanEncrypt true")
	}
	if d.active {
		t.Fatal("expected session to start with active=false")
	}
	if err := d.ActivatePreparedTransition(0); err != nil {
		t.Fatalf("ActivatePreparedTransition returned error: %v", err)
	}
	if !d.active {
		t.Fatal("expected prepared transition activation to set active")
	}
	if !d.CanEncrypt() {
		t.Fatal("expected encryption to remain enabled after activation")
	}
}

type testAEAD struct{}

func (testAEAD) NonceSize() int { return 12 }
func (testAEAD) Overhead() int  { return 16 }
func (testAEAD) Seal(dst, nonce, plaintext, additionalData []byte) []byte {
	return append(dst, plaintext...)
}
func (testAEAD) Open(dst, nonce, ciphertext, additionalData []byte) ([]byte, error) {
	return append(dst, ciphertext...), nil
}
