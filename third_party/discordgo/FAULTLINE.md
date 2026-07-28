# Faultline-patched discordgo (DAVE)

Vendored from github.com/yeongaori/discordgo-fork @ c65bda26a53b
(replace for github.com/bwmarrin/discordgo).

Local patches (see voice.go / dave.go):
- WaitForDAVEReady follows the current DAVE session (not a stale snapshot)
  and fails fast when the voice connection dies.
- CanEncrypt is true once a frame cipher exists (Welcome may leave
  `active` false; cipher is what opusSender needs).
- VOICE_SERVER_UPDATE is coalesced briefly so Discord's common double
  update does not cancel a half-open UDP/websocket handshake.
- Close 4006 surfaces as ErrVoiceSessionInvalid.

Do not bump this tree to a newer upstream without re-validating DAVE
join on a live Discord voice channel.
