package discord

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3/pkg/media/oggreader"
	"github.com/pion/webrtc/v3/pkg/media/oggwriter"
)

//go:embed assets/ack.ogg
var ackOgg []byte

const (
	voiceSilenceGate    = time.Second
	voiceMinUtterance   = 400 * time.Millisecond
	voiceMaxUtterance   = 60 * time.Second
	voiceJoinTimeout    = 20 * time.Second // includes VSU coalesce (~750ms) + DAVE Welcome
	voiceLeaveSettle    = 750 * time.Millisecond
	voiceOpusSampleRate = 48000
	voiceOpusChannels   = 2
	// Drop DAVE-decrypt garbage / accidental noise fragments before they
	// reach the agent ("20 6", single phonemes, etc.). Real short commands
	// like "yes"/"stop" still pass.
	voiceMinTranscriptRunes = 3
)

type oggSpeaker interface {
	SpeakOggOpus(ctx context.Context, text string) ([]byte, error)
}

// SetVoiceChannel enables follow-you live voice for the operator.
// Must be called before Start so gateway intents include voice states.
func (b *Bot) SetVoiceChannel(voiceChannelID, operatorUserID string) {
	b.voiceMu.Lock()
	defer b.voiceMu.Unlock()
	b.voiceChannelID = strings.TrimSpace(voiceChannelID)
	b.operatorUserID = strings.TrimSpace(operatorUserID)
	if b.voiceChannelID == "" || b.operatorUserID == "" {
		return
	}
	b.session.Identify.Intents |= discordgo.IntentGuildVoiceStates
	if !b.voiceHandlerAdded {
		b.session.AddHandler(b.onVoiceStateUpdate)
		b.session.AddHandler(b.onReadyReconcileVoice)
		b.voiceHandlerAdded = true
	}
}

func (b *Bot) voiceConfigured() bool {
	return b.speech != nil &&
		strings.TrimSpace(b.voiceChannelID) != "" &&
		strings.TrimSpace(b.operatorUserID) != ""
}

func (b *Bot) onReadyReconcileVoice(_ *discordgo.Session, _ *discordgo.Ready) {
	if !b.voiceConfigured() {
		return
	}
	go func() {
		time.Sleep(1500 * time.Millisecond)
		b.reconcileVoicePresence()
	}()
}

func (b *Bot) reconcileVoicePresence() {
	ch, err := b.session.Channel(b.voiceChannelID)
	if err != nil || ch == nil || ch.GuildID == "" {
		b.logger.Debug("voice reconcile: channel lookup failed", "error", err)
		return
	}
	g, err := b.session.State.Guild(ch.GuildID)
	if err != nil || g == nil {
		// Fall back to REST if state isn't warm yet.
		g, err = b.session.Guild(ch.GuildID)
		if err != nil || g == nil {
			b.logger.Debug("voice reconcile: guild lookup failed", "error", err)
			return
		}
	}
	for _, vs := range g.VoiceStates {
		if vs != nil && vs.UserID == b.operatorUserID && vs.ChannelID == b.voiceChannelID {
			b.joinVoice(ch.GuildID)
			return
		}
	}
}

func (b *Bot) onVoiceStateUpdate(_ *discordgo.Session, vs *discordgo.VoiceStateUpdate) {
	if vs == nil || !b.voiceConfigured() {
		return
	}
	if vs.UserID != b.operatorUserID {
		return
	}

	if vs.ChannelID == b.voiceChannelID {
		// Operator is in our VC. Join once; ignore mute/deaf/self-stream churn.
		b.joinVoice(vs.GuildID)
		return
	}

	// Operator left / moved away from our VC — only then disconnect.
	before := ""
	if vs.BeforeUpdate != nil {
		before = vs.BeforeUpdate.ChannelID
	}
	if before == b.voiceChannelID {
		b.leaveVoice()
	}
}

func voiceConnReady(vc *discordgo.VoiceConnection) bool {
	return vc != nil && vc.Status == discordgo.VoiceConnectionStatusReady
}

func voiceConnPlayable(vc *discordgo.VoiceConnection) bool {
	return vc != nil &&
		vc.Status != discordgo.VoiceConnectionStatusDead &&
		vc.OpusSend != nil
}

func (b *Bot) operatorInConfiguredVoice() bool {
	if !b.voiceConfigured() {
		return false
	}
	ch, err := b.session.Channel(b.voiceChannelID)
	if err != nil || ch == nil || ch.GuildID == "" {
		return false
	}
	g, err := b.session.State.Guild(ch.GuildID)
	if err != nil || g == nil {
		return false
	}
	for _, vs := range g.VoiceStates {
		if vs != nil && vs.UserID == b.operatorUserID && vs.ChannelID == b.voiceChannelID {
			return true
		}
	}
	return false
}

func (b *Bot) inVoiceChannel() bool {
	b.voiceMu.Lock()
	defer b.voiceMu.Unlock()
	return voiceConnReady(b.vc)
}

func (b *Bot) joinVoice(guildID string) {
	if !b.voiceConfigured() || guildID == "" {
		return
	}

	b.voiceMu.Lock()
	if voiceConnReady(b.vc) && b.voiceGuildID == guildID {
		b.voiceMu.Unlock()
		return
	}
	if b.voiceJoining {
		b.voiceMu.Unlock()
		return
	}
	b.voiceJoining = true
	old := b.vc
	b.vc = nil
	b.voiceMu.Unlock()

	defer func() {
		b.voiceMu.Lock()
		b.voiceJoining = false
		b.voiceMu.Unlock()
	}()

	if old != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = old.Disconnect(ctx)
		cancel()
		old.Kill()
	}

	// Force a gateway leave so Discord does not coalesce leave+rejoin into a
	// no-op (common cause of close 4006 on retry).
	b.forceGatewayLeave(guildID)
	if !b.operatorInConfiguredVoice() {
		b.logger.Debug("discord voice join aborted; operator left before connect")
		return
	}

	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		vc, err := b.tryJoinVoiceOnce(guildID)
		if err == nil {
			b.voiceMu.Lock()
			b.vc = vc
			b.voiceGuildID = guildID
			b.voiceMu.Unlock()
			b.logger.Info("discord voice channel joined",
				"channel_id", b.voiceChannelID,
				"guild_id", guildID,
				"operator_user_id", b.operatorUserID,
				"attempt", attempt,
			)
			go b.voiceListen(vc)
			return
		}
		lastErr = err
		b.logger.Warn("discord voice join attempt failed",
			"attempt", attempt,
			"error", err,
			"channel_id", b.voiceChannelID,
		)
		if attempt < maxAttempts {
			// 4006 / session-invalid: leave fully before the next join.
			b.forceGatewayLeave(guildID)
			time.Sleep(time.Duration(attempt) * 750 * time.Millisecond)
			if !b.operatorInConfiguredVoice() {
				b.logger.Debug("discord voice join aborted; operator left during retries")
				return
			}
		}
	}
	b.logger.Error("discord voice join failed", "error", lastErr, "channel_id", b.voiceChannelID, "attempts", maxAttempts)
}

// forceGatewayLeave clears the bot's voice state at the gateway and waits for
// Discord to settle so a subsequent join allocates a fresh session.
func (b *Bot) forceGatewayLeave(guildID string) {
	if guildID == "" || b.session == nil {
		return
	}
	if err := b.session.VoiceStateUpdate(guildID, "", true, true); err != nil {
		b.logger.Debug("discord voice gateway leave", "error", err, "guild_id", guildID)
	}
	time.Sleep(voiceLeaveSettle)
}

func (b *Bot) tryJoinVoiceOnce(guildID string) (*discordgo.VoiceConnection, error) {
	ctx, cancel := context.WithTimeout(context.Background(), voiceJoinTimeout)
	defer cancel()
	vc, err := b.session.ChannelVoiceJoin(ctx, guildID, b.voiceChannelID, false, false)
	if err != nil {
		if vc != nil {
			vc.Kill()
		}
		return nil, err
	}
	daveCtx, daveCancel := context.WithTimeout(context.Background(), voiceJoinTimeout)
	defer daveCancel()
	if err := vc.WaitForDAVEReady(daveCtx); err != nil {
		discCtx, discCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = vc.Disconnect(discCtx)
		discCancel()
		vc.Kill()
		return nil, fmt.Errorf("DAVE ready: %w", err)
	}
	if !voiceConnPlayable(vc) {
		vc.Kill()
		return nil, fmt.Errorf("voice connection not playable after join (status=%v)", vc.Status)
	}
	return vc, nil
}

func (b *Bot) leaveVoice() {
	b.voiceMu.Lock()
	if b.voiceJoining {
		// A join is in flight; don't yank the connection mid-handshake.
		b.voiceMu.Unlock()
		return
	}
	vc := b.vc
	b.vc = nil
	b.voiceGuildID = ""
	b.voiceMu.Unlock()
	if vc == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := vc.Disconnect(ctx); err != nil {
		b.logger.Debug("discord voice disconnect", "error", err)
	}
	vc.Kill()
	b.logger.Info("discord voice channel left", "channel_id", b.voiceChannelID)
}

func (b *Bot) voiceListen(vc *discordgo.VoiceConnection) {
	defer b.onVoiceListenExit(vc)

	ssrcUser := make(map[uint32]string)
	var ssrcMu sync.Mutex
	vc.AddHandler(func(_ *discordgo.VoiceConnection, vs *discordgo.VoiceSpeakingUpdate) {
		if vs == nil {
			return
		}
		ssrcMu.Lock()
		ssrcUser[uint32(vs.SSRC)] = vs.UserID
		ssrcMu.Unlock()
	})

	var (
		buf         [][]byte
		speaking    bool
		startedAt   time.Time
		lastPacket  time.Time
		silenceTick = time.NewTicker(100 * time.Millisecond)
	)
	defer silenceTick.Stop()

	finalize := func() {
		if !speaking || len(buf) == 0 {
			buf = nil
			speaking = false
			return
		}
		if time.Since(startedAt) < voiceMinUtterance {
			buf = nil
			speaking = false
			return
		}
		packets := buf
		buf = nil
		speaking = false
		go b.handleVoiceUtterance(vc, packets)
	}

	for {
		b.voiceMu.Lock()
		current := b.vc
		busy := b.voicePlaying
		b.voiceMu.Unlock()
		if current != vc || !voiceConnReady(vc) {
			return
		}

		select {
		case <-vc.Dead:
			return
		case p, ok := <-vc.OpusRecv:
			if !ok {
				return
			}
			if p == nil || busy {
				continue
			}
			ssrcMu.Lock()
			uid := ssrcUser[p.SSRC]
			ssrcMu.Unlock()
			if uid != "" && uid != b.operatorUserID {
				continue
			}
			// Until we learn the SSRC→user mapping, accept packets (personal VC).
			// Once mapped, only the operator is kept.
			if uid == "" {
				// Keep buffering only after at least one speaking update for operator,
				// or if map is empty (cold start): allow briefly.
				ssrcMu.Lock()
				knownOperator := false
				for _, u := range ssrcUser {
					if u == b.operatorUserID {
						knownOperator = true
						break
					}
				}
				ssrcMu.Unlock()
				if knownOperator {
					continue
				}
			}

			now := time.Now()
			if !speaking {
				speaking = true
				startedAt = now
				buf = nil
			}
			buf = append(buf, append([]byte(nil), p.Opus...))
			lastPacket = now
			if now.Sub(startedAt) >= voiceMaxUtterance {
				finalize()
			}

		case <-silenceTick.C:
			if speaking && !lastPacket.IsZero() && time.Since(lastPacket) >= voiceSilenceGate {
				finalize()
			}
		}
	}
}

// onVoiceListenExit clears a dead connection and rejoins if the operator is
// still in the configured VC (handles mid-call 4006 / DAVE drops).
func (b *Bot) onVoiceListenExit(vc *discordgo.VoiceConnection) {
	b.voiceMu.Lock()
	if b.vc == vc {
		b.vc = nil
		b.voiceGuildID = ""
	}
	guildID := ""
	if vc != nil {
		guildID = vc.GuildID
	}
	joining := b.voiceJoining
	b.voiceMu.Unlock()

	if joining || guildID == "" || !b.operatorInConfiguredVoice() {
		return
	}
	b.logger.Warn("discord voice connection dropped; rejoining",
		"channel_id", b.voiceChannelID,
		"guild_id", guildID,
	)
	go b.joinVoice(guildID)
}

func (b *Bot) handleVoiceUtterance(vc *discordgo.VoiceConnection, packets [][]byte) {
	if b.speech == nil || len(packets) == 0 {
		return
	}

	ogg, err := muxOpusPacketsToOgg(packets)
	if err != nil {
		b.logger.Error("voice utterance ogg mux failed", "error", err)
		return
	}

	// Ack immediately so the collaborator knows we heard them.
	if err := b.playOggOpus(vc, ackOgg); err != nil {
		b.logger.Debug("voice ack chime failed", "error", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	transcript, err := b.speech.Transcribe(ctx, ogg, "audio/ogg")
	if err != nil {
		b.logger.Error("voice channel transcription failed", "error", err)
		b.enqueue("Collaborator spoke in the voice channel but transcription failed.")
		return
	}
	transcript = strings.TrimSpace(transcript)
	if !usableVoiceTranscript(transcript) {
		if transcript != "" {
			b.logger.Debug("dropping short voice transcript", "transcript", transcript)
		}
		return
	}
	b.logger.Info("transcribed collaborator voice channel utterance", "chars", len(transcript))
	b.enqueue(messaging.VoiceChannelPreamble + transcript)
}

func usableVoiceTranscript(transcript string) bool {
	return utf8.RuneCountInString(transcript) >= voiceMinTranscriptRunes
}

func muxOpusPacketsToOgg(packets [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := oggwriter.NewWith(&buf, voiceOpusSampleRate, voiceOpusChannels)
	if err != nil {
		return nil, err
	}
	var seq uint16
	var ts uint32 = 1
	for _, opus := range packets {
		if len(opus) == 0 {
			continue
		}
		pkt := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    0x78,
				SequenceNumber: seq,
				Timestamp:      ts,
				SSRC:           1,
			},
			Payload: opus,
		}
		seq++
		ts += 960
		if err := w.WriteRTP(pkt); err != nil {
			_ = w.Close()
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if buf.Len() == 0 {
		return nil, fmt.Errorf("empty ogg")
	}
	return buf.Bytes(), nil
}

func (b *Bot) playOggOpus(vc *discordgo.VoiceConnection, ogg []byte) error {
	if !voiceConnReady(vc) || vc.OpusSend == nil {
		return fmt.Errorf("voice connection not ready")
	}
	reader, _, err := oggreader.NewWith(bytes.NewReader(ogg))
	if err != nil {
		return err
	}

	b.voiceMu.Lock()
	b.voicePlaying = true
	b.voiceMu.Unlock()
	defer func() {
		b.voiceMu.Lock()
		b.voicePlaying = false
		b.voiceMu.Unlock()
		_ = vc.Speaking(false)
	}()

	if err := vc.Speaking(true); err != nil {
		return err
	}

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	pages := 0
	for {
		page, _, err := reader.ParseNextPage()
		if err == io.EOF {
			if pages == 0 {
				return fmt.Errorf("ogg opus has no audio pages")
			}
			return nil
		}
		if err != nil {
			return err
		}
		if len(page) == 0 {
			continue
		}
		<-ticker.C
		b.voiceMu.Lock()
		still := b.vc == vc
		b.voiceMu.Unlock()
		if !still {
			return fmt.Errorf("left voice channel during playback")
		}
		select {
		case vc.OpusSend <- page:
			pages++
		case <-time.After(2 * time.Second):
			return fmt.Errorf("opus send timed out")
		}
	}
}

// SendVoice synthesizes speech and plays it in the voice channel when the
// operator is in the configured VC (or the bot is already connected).
// Otherwise it uploads an audio attachment to the text channel.
func (b *Bot) SendVoice(text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("text is required")
	}
	if b.speech == nil {
		return fmt.Errorf("voice replies require [deepgram] to be configured")
	}

	spoken := stripForSpeech(text)
	if spoken == "" {
		spoken = text
	}

	b.voiceMu.Lock()
	vc := b.vc
	status := discordgo.VoiceConnectionStatusDead
	if vc != nil {
		status = vc.Status
	}
	b.voiceMu.Unlock()

	wantVC := voiceConnPlayable(vc) || b.operatorInConfiguredVoice()
	if wantVC {
		if !voiceConnPlayable(vc) {
			return fmt.Errorf("operator is in the voice channel but the bot is not connected (status=%v); cannot play spoken reply", status)
		}
		oggSp, ok := b.speech.(oggSpeaker)
		if !ok {
			return fmt.Errorf("voice channel TTS requires Ogg/Opus (SpeakOggOpus) support")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		ogg, err := oggSp.SpeakOggOpus(ctx, spoken)
		if err != nil {
			return fmt.Errorf("tts ogg: %w", err)
		}
		b.logger.Info("playing voice reply in voice channel", "bytes", len(ogg), "chars", len(spoken))
		if err := b.playOggOpus(vc, ogg); err != nil {
			return fmt.Errorf("play voice reply: %w", err)
		}
		return nil
	}

	oggSp, ok := b.speech.(oggSpeaker)
	if !ok {
		return fmt.Errorf("discord voice messages require Ogg/Opus (SpeakOggOpus) support")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	ogg, err := oggSp.SpeakOggOpus(ctx, spoken)
	if err != nil {
		return fmt.Errorf("tts ogg: %w", err)
	}

	duration := oggOpusDurationSecs(ogg)
	waveform := waveformFromBytes(ogg)
	b.logger.Info("sending discord voice message", "bytes", len(ogg), "chars", len(spoken), "duration_s", duration)
	_, err = b.session.ChannelMessageSendComplex(b.channelID, &discordgo.MessageSend{
		Flags: discordgo.MessageFlagsIsVoiceMessage,
		Files: []*discordgo.File{{
			Name:        "voice-message.ogg",
			ContentType: "audio/ogg",
			Reader:      bytes.NewReader(ogg),
		}},
		Attachments: []*discordgo.MessageAttachment{{
			ID:           "0",
			Filename:     "voice-message.ogg",
			DurationSecs: duration,
			Waveform:     waveform,
		}},
	})
	if err != nil {
		return fmt.Errorf("send discord voice message: %w", err)
	}
	return nil
}
