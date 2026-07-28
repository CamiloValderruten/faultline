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
)

func buildComponents(buttons [][]messaging.Button, selects []messaging.SelectMenu) ([]discordgo.MessageComponent, error) {
	var rows []discordgo.MessageComponent

	for i, row := range buttons {
		if len(row) == 0 {
			return nil, fmt.Errorf("buttons row %d is empty", i)
		}
		if len(row) > maxButtonsPerRow {
			return nil, fmt.Errorf("buttons row %d has more than %d buttons", i, maxButtonsPerRow)
		}
		var comps []discordgo.MessageComponent
		for j, b := range row {
			btn, err := toDiscordButton(b, i, j)
			if err != nil {
				return nil, err
			}
			comps = append(comps, btn)
		}
		rows = append(rows, discordgo.ActionsRow{Components: comps})
	}

	for i, sel := range selects {
		menu, err := toDiscordSelect(sel, i)
		if err != nil {
			return nil, err
		}
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}})
	}

	if len(rows) > maxActionRows {
		return nil, fmt.Errorf("at most %d action rows allowed (button rows + selects)", maxActionRows)
	}
	return rows, nil
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
	if len(sel.Options) == 0 {
		return discordgo.SelectMenu{}, fmt.Errorf("selects[%d].options is required", idx)
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
	return discordgo.SelectMenu{
		CustomID:    id,
		Placeholder: strings.TrimSpace(sel.Placeholder),
		Options:     opts,
	}, nil
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
