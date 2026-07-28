# Faultline-patched discordgo

Vendored replace for `github.com/bwmarrin/discordgo` (yeongaori DAVE fork base).

## Why this tree exists

`MessageSend.Attachments` carries Discord voice-message multipart metadata
(`duration_secs` / `waveform`) used by text-channel voice-note replies.

## Live voice channel (archived)

Follow-you VC / DAVE join work was abandoned. Full history lives on branch
`archive/discord-live-voice`. Adapter-side live VC is removed from `main`;
unused DAVE helpers may remain in this tree until the fork is slimmed.
