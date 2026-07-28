module github.com/CamiloValderruten/faultline

go 1.26.1

require (
	github.com/BrianLeishman/go-imap v0.1.28
	github.com/BurntSushi/toml v1.6.0
	github.com/Mad-Pixels/goldmark-tgmd v0.0.10
	github.com/bwmarrin/discordgo v0.29.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/pion/webrtc/v3 v3.3.5
	golang.org/x/crypto v0.50.0
	golang.org/x/mod v0.35.0
	golang.org/x/net v0.53.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/FlameInTheDark/go-dave v1.1.0 // indirect
	github.com/StirlingMarketingGroup/go-retry v0.0.0-20190512160921-94a8eb23e893 // indirect
	github.com/cention-sany/utf7 v0.0.0-20170124080048-26cad61bd60a // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/inbucket/html2text v1.0.0 // indirect
	github.com/jhillyerd/enmime/v2 v2.3.0 // indirect
	github.com/kr/pretty v0.1.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.21 // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.2.0 // indirect
	github.com/olekukonko/ll v0.1.8 // indirect
	github.com/olekukonko/tablewriter v1.1.4 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/sqs/go-xoauth2 v0.0.0-20120917012134-0911dad68e56 // indirect
	github.com/ssor/bom v0.0.0-20170718123548-6386211fdfcf // indirect
	github.com/thomas-vilte/mls-go v1.0.0 // indirect
	github.com/yuin/goldmark v1.6.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
)

// Discord enforces DAVE/E2EE on voice (close 4017). Upstream discordgo lacks
// it; we vendor yeongaori's DAVE fork with Faultline patches for the
// WaitForDAVEReady session-pointer race, VOICE_SERVER_UPDATE coalesce, and
// close-4006 handling. See third_party/discordgo/FAULTLINE.md.
replace github.com/bwmarrin/discordgo => ./third_party/discordgo
