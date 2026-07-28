package discordgo

import (
	"fmt"
	"sync"

	godave "github.com/FlameInTheDark/go-dave"
)

type DAVESession struct {
	mu             sync.Mutex
	session        *godave.DAVESession
	userID         string
	channelID      string
	ssrcToUser     map[uint32]string
	pendingVersion int
}

func NewDAVESession(userID, channelID string, protocolVersion int) (*DAVESession, error) {
	session, err := godave.NewDAVESession(uint16(protocolVersion), userID, channelID, nil)
	if err != nil {
		return nil, err
	}
	return &DAVESession{
		session:    session,
		userID:     userID,
		channelID:  channelID,
		ssrcToUser: make(map[uint32]string),
	}, nil
}

func (d *DAVESession) GenerateKeyPackage() ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	packet, err := d.session.GetKeyPackagePacket()
	if err != nil {
		return nil, err
	}
	return packet[1:], nil
}

func (d *DAVESession) HandleGatewayBinary(message []byte) (*godave.GatewayBinaryResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.session.HandleGatewayBinaryPacket(message, nil)
}

func (d *DAVESession) HandlePrepareTransition(_ uint16, protocolVersion int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.pendingVersion = protocolVersion
	if protocolVersion == 0 {
		d.session.SetPassthroughMode(true)
	}
}

func (d *DAVESession) HandleExecuteTransition(_ uint16) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pendingVersion > 0 {
		d.session.SetPassthroughMode(false)
	}
	return nil
}

func (d *DAVESession) HandlePrepareEpoch(epoch uint64, protocolVersion int) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if epoch != 1 {
		return nil, nil
	}
	if err := d.session.Reinit(uint16(protocolVersion), d.userID, d.channelID, nil); err != nil {
		return nil, err
	}
	packet, err := d.session.GetKeyPackagePacket()
	if err != nil {
		return nil, err
	}
	return packet[1:], nil
}

func (d *DAVESession) EncryptFrame(opusData []byte) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.session.EncryptOpus(opusData)
}

func (d *DAVESession) SetSSRC(ssrc uint32, userID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ssrcToUser[ssrc] = userID
}

func (d *DAVESession) DecryptFrame(ssrc uint32, data []byte) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	userID := d.ssrcToUser[ssrc]
	if userID == "" {
		return nil, fmt.Errorf("unknown SSRC %d", ssrc)
	}
	return d.session.Decrypt(userID, godave.MediaTypeAudio, data)
}

func (d *DAVESession) CanEncrypt() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.session.Ready()
}

func (d *DAVESession) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	_ = d.session.Reset()
}
