# Faultline-patched discordgo (DAVE)

Vendored from github.com/yeongaori/discordgo-fork @ c65bda26a53b
(replace for github.com/bwmarrin/discordgo).

## DAVE MLS

Handshake crypto is delegated to `github.com/FlameInTheDark/go-dave`
(pure Go via `thomas-vilte/mls-go`). `dave.go` is a thin wrapper:

- OP25 external sender → pending solo group
- OP27 proposals → OP28 commit_welcome
- OP29/OP30 commit/welcome → ratchets / `Ready()`

`HandleGatewayBinary` passes `recognizedUserIDs=nil` so proposal
allowlisting is skipped (incomplete SSRC rosters would otherwise reject
valid Add proposals).

## Local patches (voice.go / dave.go)

- WaitForDAVEReady follows the current DAVE session (not a stale snapshot)
  and fails fast when the voice connection dies.
- Close codes 4014/4017/4021/4022 fail WaitForDAVEReady immediately;
  4006 surfaces as ErrVoiceSessionInvalid.
- MessageSend.Attachments supports voice-message duration_secs / waveform
  metadata on multipart uploads.

Do not bump this tree to a newer upstream without re-validating DAVE
join on a live Discord voice channel.
