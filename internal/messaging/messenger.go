// Package messaging holds the collaborator outbound message shapes shared
// by operator adapters (Telegram, Discord) and the tools layer.
package messaging

import "strings"

// Messenger is the tools-side collaborator send port. Adapters that also
// satisfy agent.Operator (Pending) are wired as both operator and messenger.
// HasPending lets sleep/subagent_wait wake without draining the queue.
// ChannelGuide returns runtime-injected collaborator-channel instructions
// for the system prompt (empty string is never used — adapters always
// return a non-empty guide).
type Messenger interface {
	Send(text string) error
	SendWithButtons(text string, buttons [][]Button) error
	SendRich(msg RichMessage) error
	HasPending() bool
	ChannelGuide() string
}

// Button is one interactive button. Data is the opaque callback id returned
// when pressed (Telegram callback_data / Discord custom_id). Style and URL
// are honored by Discord; Telegram ignores Style and uses URL for link
// buttons when set.
//
// Modal (Discord only): when set, pressing the button opens a popup form
// instead of enqueueing a button press. The adapter must respond within
// Discord's 3s interaction window, so the agent declares the modal on
// send — it cannot open one later via a tool. Modal submit arrives as a
// collaborator message. Telegram ignores Modal.
type Button struct {
	Text  string     `json:"text"`
	Data  string     `json:"data"`
	Style string     `json:"style,omitempty"` // primary|secondary|success|danger|link
	URL   string     `json:"url,omitempty"`
	Modal *ModalSpec `json:"modal,omitempty"`
}

// ModalSpec describes a Discord modal opened by a button press.
type ModalSpec struct {
	ID     string       `json:"id"` // custom_id on the modal submit
	Title  string       `json:"title"`
	Fields []ModalField `json:"fields"`
}

// ModalField is one text input in a ModalSpec.
type ModalField struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Style       string `json:"style,omitempty"` // short|paragraph (default short)
	Placeholder string `json:"placeholder,omitempty"`
	Required    bool   `json:"required,omitempty"`
	MinLength   int    `json:"min_length,omitempty"`
	MaxLength   int    `json:"max_length,omitempty"`
	Value       string `json:"value,omitempty"` // prefill
}

// SelectMenu is a dropdown. Discord renders it; Telegram flattens options
// into the message text.
//
// Type selects Discord auto-populated menus: string (default), user, role,
// channel, mentionable. Options are required only for string selects.
type SelectMenu struct {
	ID          string         `json:"id"`
	Placeholder string         `json:"placeholder,omitempty"`
	Type        string         `json:"type,omitempty"` // string|user|role|channel|mentionable
	MinValues   int            `json:"min_values,omitempty"`
	MaxValues   int            `json:"max_values,omitempty"`
	Options     []SelectOption `json:"options,omitempty"`
}

// SelectOption is one choice in a SelectMenu.
type SelectOption struct {
	Label       string `json:"label"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// RichField is one embed field (Discord) or a markdown section (Telegram).
type RichField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// RichMessage is a structured collaborator digest. Adapters render what
// they can and degrade the rest (Telegram: markdown + buttons; Discord:
// embed + components).
type RichMessage struct {
	Content string       `json:"content"`
	Title   string       `json:"title,omitempty"`
	Color   int          `json:"color,omitempty"` // Discord embed color (0 = default)
	Fields  []RichField  `json:"fields,omitempty"`
	Buttons [][]Button   `json:"buttons,omitempty"`
	Selects []SelectMenu `json:"selects,omitempty"`
}

// FlattenText builds a markdown body from Content/Title/Fields/Selects for
// adapters that cannot render structured embeds or select menus.
func FlattenText(msg RichMessage) string {
	var b strings.Builder
	if title := strings.TrimSpace(msg.Title); title != "" {
		b.WriteString("**")
		b.WriteString(title)
		b.WriteString("**\n\n")
	}
	if content := strings.TrimSpace(msg.Content); content != "" {
		b.WriteString(content)
	}
	for _, f := range msg.Fields {
		name := strings.TrimSpace(f.Name)
		value := strings.TrimSpace(f.Value)
		if name == "" && value == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		if name != "" {
			b.WriteString("**")
			b.WriteString(name)
			b.WriteString("**\n")
		}
		b.WriteString(value)
	}
	for _, sel := range msg.Selects {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		ph := strings.TrimSpace(sel.Placeholder)
		if ph == "" {
			ph = strings.TrimSpace(sel.ID)
		}
		if ph == "" {
			ph = "options"
		}
		b.WriteString("*")
		b.WriteString(ph)
		b.WriteString(":*\n")
		typ := strings.ToLower(strings.TrimSpace(sel.Type))
		if typ != "" && typ != "string" {
			b.WriteString("(Discord ")
			b.WriteString(typ)
			b.WriteString(" picker)\n")
			continue
		}
		for _, opt := range sel.Options {
			b.WriteString("- ")
			b.WriteString(strings.TrimSpace(opt.Label))
			if v := strings.TrimSpace(opt.Value); v != "" && v != strings.TrimSpace(opt.Label) {
				b.WriteString(" (`")
				b.WriteString(v)
				b.WriteString("`)")
			}
			if d := strings.TrimSpace(opt.Description); d != "" {
				b.WriteString(" — ")
				b.WriteString(d)
			}
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}
