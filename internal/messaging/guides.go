package messaging

// Runtime-injected collaborator channel guides. Kept here (not in
// prompts/templates) so the active channel is decided by composition,
// not by editable prompt files.

// TelegramChannelGuide is injected when Telegram is the collaborator channel.
const TelegramChannelGuide = `## Collaborator Channel

You are talking to your collaborator over **Telegram**.

- Ordinary chat: **send_message(text)**. Markdown is fine.
- **Always deliver a reply** with send_message / send_rich_message — assistant text never reaches Telegram. Use tools first if you need them, then send the answer. For long work, an optional short real acknowledgment is fine — not required for simple answers.
- Decisions / approvals: optional inline **buttons** on send_message — rows of {text, data} (optional url for link buttons). Use sparingly: a few clear choices, not a control panel. When they tap a button you receive a collaborator message with the button data.
- Digests / status cards: prefer **send_rich_message(content)** (headings and lists). You can also pass title / fields; they flatten to Markdown on Telegram. Buttons on a rich message work the same as on send_message.
- Telegram has no select menus — if you pass selects, they become a text list. Prefer buttons or plain questions instead.
- Keep messages readable on a phone; chunk long updates rather than one wall of text.
`

// DiscordChannelGuide is injected when Discord is the collaborator channel.
const DiscordChannelGuide = `## Collaborator Channel

You are talking to your collaborator over **Discord**.

- Ordinary chat: **send_message(text)**. Discord Markdown is fine.
- **Always deliver a reply** with send_message / send_rich_message / send_voice_message — assistant text never reaches Discord. Use tools first if you need them, then send the answer. For long work, an optional short real acknowledgment is fine — not required for simple answers.
- Decisions / approvals: optional **buttons** (rows of {text, data, style?, url?, modal?}; styles: primary|secondary|success|danger|link) and optional **selects** (dropdowns: {id, placeholder?, type?, options?}). Select type is string (default; needs options) or user|role|channel|mentionable (Discord auto-populated; leave options empty). Use sparingly — a small set of clear actions, not a dense UI. After they click a normal button/select, controls on that message disable; you receive a collaborator message with the choice.
- **Modals (popup forms):** attach modal: {id, title, fields:[{id,label,style?,required?}]} on a button. When they press it, Discord opens the form immediately (you cannot open a modal later via a tool). On submit you receive a collaborator message like Modal "id" submitted: field="value" …. Use for structured Q&A (feeding log, mood check, preferences).
- Digests / status cards: prefer **send_rich_message** with content plus optional title, color (integer embed color), and fields ({name, value, inline?}). Discord renders this as an embed; you can attach the same buttons/selects.
- Example rich digest: title "Evening status", a short content summary, two fields ("Home", "Kids"), and one row of Approve / Need more info buttons — not five rows of buttons.
- Prefer embeds for structured updates; prefer plain send_message for conversation.
- Files: **send_file(path, filename?, text?)** uploads a sandbox file (/output/…, /input/…, /scripts/…) to the channel (images, PDFs, audio).
- Voice notes (text channel): when the collaborator sends a voice note you receive a transcript marked as a voice note. Prefer a short spoken reply via **send_voice_message(text)** — it sends a Discord voice-message bubble. Use send_message for longer detail if needed.
`

// VoiceNotePreamble prefixes STT transcripts enqueued from collaborator voice notes.
const VoiceNotePreamble = `[Collaborator voice note — reply briefly in spoken language via send_voice_message; you may take your time to think and use tools before answering.]

Transcript: `
