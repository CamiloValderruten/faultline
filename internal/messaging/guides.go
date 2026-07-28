package messaging

// Runtime-injected collaborator channel guides. Kept here (not in
// prompts/templates) so the active channel is decided by composition,
// not by editable prompt files.

// TelegramChannelGuide is injected when Telegram is the collaborator channel.
const TelegramChannelGuide = `## Collaborator Channel

You are talking to your collaborator over **Telegram**.

- Ordinary chat: **send_message(text)**. Markdown is fine.
- Decisions / approvals: optional inline **buttons** on send_message — rows of {text, data} (optional url for link buttons). Use sparingly: a few clear choices, not a control panel. When they tap a button you receive a collaborator message with the button data.
- Digests / status cards: prefer **send_rich_message(content)** (headings and lists). You can also pass title / fields; they flatten to Markdown on Telegram. Buttons on a rich message work the same as on send_message.
- Telegram has no select menus — if you pass selects, they become a text list. Prefer buttons or plain questions instead.
- Keep messages readable on a phone; chunk long updates rather than one wall of text.
`

// DiscordChannelGuide is injected when Discord is the collaborator channel.
const DiscordChannelGuide = `## Collaborator Channel

You are talking to your collaborator over **Discord**.

- Ordinary chat: **send_message(text)**. Discord Markdown is fine.
- Decisions / approvals: optional **buttons** (rows of {text, data, style?, url?}; styles: primary|secondary|success|danger|link) and optional **selects** (dropdowns: {id, placeholder?, options:[{label,value,description?}]}). Use sparingly — a small set of clear actions, not a dense UI. After they click, controls on that message disable; you receive a collaborator message with the choice.
- Digests / status cards: prefer **send_rich_message** with content plus optional title, color (integer embed color), and fields ({name, value, inline?}). Discord renders this as an embed; you can attach the same buttons/selects.
- Example rich digest: title "Evening status", a short content summary, two fields ("Home", "Kids"), and one row of Approve / Need more info buttons — not five rows of buttons.
- Prefer embeds for structured updates; prefer plain send_message for conversation.
- Voice notes (text channel): when the collaborator sends a voice note you receive a transcript marked as a voice note. Prefer a short spoken reply via **send_voice_message(text)**.
- Live voice channel: when they speak in the configured voice channel you receive a transcript marked as a voice-channel utterance. Prefer a short spoken reply via **send_voice_message(text)** — it plays in the voice channel while they are there. You may take time to think and use tools; they hear an ack chime when you received the utterance. Use send_message for longer detail if needed.
`

// VoiceNotePreamble prefixes STT transcripts enqueued from collaborator voice notes.
const VoiceNotePreamble = `[Collaborator voice note — reply briefly in spoken language via send_voice_message; you may take your time to think and use tools before answering.]

Transcript: `

// VoiceChannelPreamble prefixes STT transcripts from live voice-channel utterances.
const VoiceChannelPreamble = `[Collaborator voice channel — reply briefly in spoken language via send_voice_message (plays in the VC); you may take your time to think and use tools before answering.]

Transcript: `
