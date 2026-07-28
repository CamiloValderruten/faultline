package discord

import (
	"fmt"
	"strings"

	"github.com/CamiloValderruten/faultline/internal/messaging"
	"github.com/bwmarrin/discordgo"
)

const (
	maxActionRows     = 5
	maxButtonsPerRow  = 5
	maxSelectOptions  = 25
	maxCustomIDLen    = 100
	maxButtonLabelLen = 80
	maxModalFields    = 5
	maxModalTitleLen  = 45
	maxModalLabelLen  = 45
)

func buildComponents(buttons [][]messaging.Button, selects []messaging.SelectMenu) ([]discordgo.MessageComponent, map[string]messaging.ModalSpec, error) {
	var rows []discordgo.MessageComponent
	modals := map[string]messaging.ModalSpec{}

	for i, row := range buttons {
		if len(row) == 0 {
			return nil, nil, fmt.Errorf("buttons row %d is empty", i)
		}
		if len(row) > maxButtonsPerRow {
			return nil, nil, fmt.Errorf("buttons row %d has more than %d buttons", i, maxButtonsPerRow)
		}
		var comps []discordgo.MessageComponent
		for j, b := range row {
			btn, err := toDiscordButton(b, i, j)
			if err != nil {
				return nil, nil, err
			}
			comps = append(comps, btn)
			if b.Modal != nil {
				id := strings.TrimSpace(b.Data)
				spec, err := validateModal(*b.Modal, i, j)
				if err != nil {
					return nil, nil, err
				}
				if id == "" {
					return nil, nil, fmt.Errorf("buttons[%d][%d].data is required when modal is set", i, j)
				}
				modals[id] = spec
			}
		}
		rows = append(rows, discordgo.ActionsRow{Components: comps})
	}

	for i, sel := range selects {
		menu, err := toDiscordSelect(sel, i)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}})
	}

	if len(rows) > maxActionRows {
		return nil, nil, fmt.Errorf("at most %d action rows allowed (button rows + selects)", maxActionRows)
	}
	return rows, modals, nil
}

func validateModal(m messaging.ModalSpec, row, col int) (messaging.ModalSpec, error) {
	id := strings.TrimSpace(m.ID)
	title := strings.TrimSpace(m.Title)
	if id == "" {
		return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.id is required", row, col)
	}
	if len(id) > maxCustomIDLen {
		return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.id exceeds %d characters", row, col, maxCustomIDLen)
	}
	if title == "" {
		return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.title is required", row, col)
	}
	if len(title) > maxModalTitleLen {
		return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.title exceeds %d characters", row, col, maxModalTitleLen)
	}
	if len(m.Fields) == 0 {
		return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.fields is required", row, col)
	}
	if len(m.Fields) > maxModalFields {
		return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal has more than %d fields", row, col, maxModalFields)
	}
	out := messaging.ModalSpec{ID: id, Title: title, Fields: make([]messaging.ModalField, 0, len(m.Fields))}
	seen := map[string]bool{}
	for k, f := range m.Fields {
		fid := strings.TrimSpace(f.ID)
		label := strings.TrimSpace(f.Label)
		if fid == "" {
			return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.fields[%d].id is required", row, col, k)
		}
		if len(fid) > maxCustomIDLen {
			return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.fields[%d].id exceeds %d characters", row, col, k, maxCustomIDLen)
		}
		if seen[fid] {
			return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.fields[%d].id %q is duplicated", row, col, k, fid)
		}
		seen[fid] = true
		if label == "" {
			return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.fields[%d].label is required", row, col, k)
		}
		if len(label) > maxModalLabelLen {
			return messaging.ModalSpec{}, fmt.Errorf("buttons[%d][%d].modal.fields[%d].label exceeds %d characters", row, col, k, maxModalLabelLen)
		}
		out.Fields = append(out.Fields, messaging.ModalField{
			ID:          fid,
			Label:       label,
			Style:       strings.TrimSpace(f.Style),
			Placeholder: strings.TrimSpace(f.Placeholder),
			Required:    f.Required,
			MinLength:   f.MinLength,
			MaxLength:   f.MaxLength,
			Value:       f.Value,
		})
	}
	return out, nil
}

func modalResponse(spec messaging.ModalSpec) (*discordgo.InteractionResponse, error) {
	rows := make([]discordgo.MessageComponent, 0, len(spec.Fields))
	for _, f := range spec.Fields {
		style := discordgo.TextInputShort
		if strings.EqualFold(f.Style, "paragraph") {
			style = discordgo.TextInputParagraph
		}
		rows = append(rows, discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.TextInput{
					CustomID:    f.ID,
					Label:       f.Label,
					Style:       style,
					Placeholder: f.Placeholder,
					Required:    f.Required,
					MinLength:   f.MinLength,
					MaxLength:   f.MaxLength,
					Value:       f.Value,
				},
			},
		})
	}
	return &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID:   spec.ID,
			Title:      spec.Title,
			Components: rows,
		},
	}, nil
}

func toDiscordButton(b messaging.Button, row, col int) (discordgo.Button, error) {
	label := strings.TrimSpace(b.Text)
	if label == "" {
		return discordgo.Button{}, fmt.Errorf("buttons[%d][%d].text is required", row, col)
	}
	if len(label) > maxButtonLabelLen {
		return discordgo.Button{}, fmt.Errorf("buttons[%d][%d].text exceeds %d characters", row, col, maxButtonLabelLen)
	}

	style := buttonStyle(b.Style, b.URL)
	url := strings.TrimSpace(b.URL)
	id := strings.TrimSpace(b.Data)

	if style == discordgo.LinkButton {
		if url == "" {
			return discordgo.Button{}, fmt.Errorf("buttons[%d][%d].url is required for link style", row, col)
		}
		if b.Modal != nil {
			return discordgo.Button{}, fmt.Errorf("buttons[%d][%d]: link buttons cannot open modals", row, col)
		}
		return discordgo.Button{Label: label, Style: style, URL: url}, nil
	}
	if id == "" {
		return discordgo.Button{}, fmt.Errorf("buttons[%d][%d].data is required", row, col)
	}
	if len(id) > maxCustomIDLen {
		return discordgo.Button{}, fmt.Errorf("buttons[%d][%d].data exceeds %d characters", row, col, maxCustomIDLen)
	}
	return discordgo.Button{Label: label, Style: style, CustomID: id}, nil
}

func buttonStyle(style, url string) discordgo.ButtonStyle {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "primary", "blurple":
		return discordgo.PrimaryButton
	case "secondary", "grey", "gray":
		return discordgo.SecondaryButton
	case "success", "green":
		return discordgo.SuccessButton
	case "danger", "red":
		return discordgo.DangerButton
	case "link":
		return discordgo.LinkButton
	default:
		if strings.TrimSpace(url) != "" {
			return discordgo.LinkButton
		}
		return discordgo.SecondaryButton
	}
}

func toDiscordSelect(sel messaging.SelectMenu, idx int) (discordgo.SelectMenu, error) {
	id := strings.TrimSpace(sel.ID)
	if id == "" {
		return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].id is required", idx)
	}
	if len(id) > maxCustomIDLen {
		return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].id exceeds %d characters", idx, maxCustomIDLen)
	}

	menuType, err := selectMenuType(sel.Type, idx)
	if err != nil {
		return discordgo.SelectMenu{}, err
	}

	menu := discordgo.SelectMenu{
		MenuType:    menuType,
		CustomID:    id,
		Placeholder: strings.TrimSpace(sel.Placeholder),
	}
	if sel.MinValues > 0 {
		min := sel.MinValues
		menu.MinValues = &min
	}
	if sel.MaxValues > 0 {
		menu.MaxValues = sel.MaxValues
	}

	if menuType == discordgo.StringSelectMenu {
		if len(sel.Options) == 0 {
			return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].options is required for string selects", idx)
		}
		if len(sel.Options) > maxSelectOptions {
			return discordgo.SelectMenu{}, fmt.Errorf("selects[%d] has more than %d options", idx, maxSelectOptions)
		}
		opts := make([]discordgo.SelectMenuOption, 0, len(sel.Options))
		for j, opt := range sel.Options {
			label := strings.TrimSpace(opt.Label)
			value := strings.TrimSpace(opt.Value)
			if label == "" {
				return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].options[%d].label is required", idx, j)
			}
			if value == "" {
				return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].options[%d].value is required", idx, j)
			}
			opts = append(opts, discordgo.SelectMenuOption{
				Label:       label,
				Value:       value,
				Description: strings.TrimSpace(opt.Description),
			})
		}
		menu.Options = opts
	} else if len(sel.Options) > 0 {
		return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].options must be empty for %s selects", idx, sel.Type)
	}

	return menu, nil
}

func selectMenuType(typ string, idx int) (discordgo.SelectMenuType, error) {
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "", "string":
		return discordgo.StringSelectMenu, nil
	case "user":
		return discordgo.UserSelectMenu, nil
	case "role":
		return discordgo.RoleSelectMenu, nil
	case "channel":
		return discordgo.ChannelSelectMenu, nil
	case "mentionable":
		return discordgo.MentionableSelectMenu, nil
	default:
		return 0, fmt.Errorf("selects[%d].type must be string|user|role|channel|mentionable", idx)
	}
}

// disableMessageComponents returns a copy of message components with every
// interactive control Disabled. Used as the interaction ack so the collaborator
// sees the click land and cannot double-press.
func disableMessageComponents(comps []discordgo.MessageComponent) []discordgo.MessageComponent {
	if len(comps) == 0 {
		return nil
	}
	out := make([]discordgo.MessageComponent, 0, len(comps))
	for _, c := range comps {
		switch row := c.(type) {
		case discordgo.ActionsRow:
			out = append(out, disableActionsRow(row))
		case *discordgo.ActionsRow:
			if row != nil {
				out = append(out, disableActionsRow(*row))
			}
		default:
			out = append(out, c)
		}
	}
	return out
}

func disableActionsRow(row discordgo.ActionsRow) discordgo.ActionsRow {
	children := make([]discordgo.MessageComponent, 0, len(row.Components))
	for _, child := range row.Components {
		switch c := child.(type) {
		case discordgo.Button:
			c.Disabled = true
			children = append(children, c)
		case *discordgo.Button:
			if c == nil {
				continue
			}
			btn := *c
			btn.Disabled = true
			children = append(children, btn)
		case discordgo.SelectMenu:
			c.Disabled = true
			children = append(children, c)
		case *discordgo.SelectMenu:
			if c == nil {
				continue
			}
			menu := *c
			menu.Disabled = true
			children = append(children, menu)
		default:
			children = append(children, child)
		}
	}
	return discordgo.ActionsRow{Components: children}
}

func formatModalPending(data discordgo.ModalSubmitInteractionData) string {
	id := strings.TrimSpace(data.CustomID)
	var parts []string
	for _, row := range data.Components {
		var comps []discordgo.MessageComponent
		switch r := row.(type) {
		case discordgo.ActionsRow:
			comps = r.Components
		case *discordgo.ActionsRow:
			if r != nil {
				comps = r.Components
			}
		}
		for _, c := range comps {
			ti, ok := c.(*discordgo.TextInput)
			if !ok {
				if v, ok2 := c.(discordgo.TextInput); ok2 {
					ti = &v
				} else {
					continue
				}
			}
			parts = append(parts, fmt.Sprintf("%s=%q", strings.TrimSpace(ti.CustomID), ti.Value))
		}
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Modal %q submitted (no fields)", id)
	}
	return fmt.Sprintf("Modal %q submitted: %s", id, strings.Join(parts, " "))
}
