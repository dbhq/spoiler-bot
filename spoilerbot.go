package spoilerbot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/bwmarrin/discordgo"
	humanize "github.com/dustin/go-humanize"
)

// Toml config
type config struct {
	Token      string `toml:"token"`
	ClientID   string `toml:"client_id"`
	BotName    string `toml:"bot_name"`
	EmbedColor int    `toml:"embed_color"`
}

type paste struct {
	// Struct for paste json response
	Paste struct {
		ID             string    `json:"id"`
		Link           string    `json:"link"`
		Raw            string    `json:"raw"`
		LangCode       string    `json:"lang_code"`
		Formatted      string    `json:"formatted"`
		ExpirationDate time.Time `json:"expiration_date"`
	} `json:"paste"`
	Status string `json:"status"`
}

func pasteURL(title string, text string) string {
	form := url.Values{
		"title": {title},
		"lang":  {"markdown"},
		"paste": {text},
	}
	encoded := bytes.NewBufferString(form.Encode())

	client := &http.Client{}
	request, _ := http.NewRequest("POST", "https://mnn.im/c", encoded)
	response, _ := client.Do(request)
	body, _ := ioutil.ReadAll(response.Body)
	defer response.Body.Close()

	var p paste
	json.Unmarshal(body, &p)
	return p.Paste.Formatted
}

func main() {
	// Load toml config
	var c config
	if _, err := toml.DecodeFile("./config.toml", &c); err != nil {
		fmt.Println("error loading config.toml,", err)
		return
	}

	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + c.Token)
	if err != nil {
		fmt.Println("error creating Discord session,", err)
		return
	}

	// Register the messageCreate func as a callback for MessageCreate events.
	dg.AddHandler(messageCreate)

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("error opening connection,", err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) every time a new
// message is created on any channel that the authenticated bot has access to.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate, c *config) {
	// Ignore all messages created by the bot itself
	// This isn't required in this specific example but it's a good practice.
	if m.Author.ID == s.State.User.ID {
		return
	}

	message := strings.ToLower(m.Content)

	// Spoiler command
	cw := regexp.MustCompile(`^!(sp|cw) (.+)\|(.+)`) // has warning
	nw := regexp.MustCompile(`^!(sp|cw) (.+)`)       // no warning
	if cw.MatchString(message) {
		cmdSpoiler(s, m, c, true)
	} else if nw.MatchString(message) {
		cmdSpoiler(s, m, c, false)
	}

	if message == "!help" {
		cmdHelp(s, m, c)
	}

	if message == "!stats" {
		cmdStats(s, m, c)
	}

	if message == "!ping" {
		s.ChannelMessageSend(m.ChannelID, "Pong!")
	}
}

func cmdSpoiler(s *discordgo.Session, m *discordgo.MessageCreate, c *config, hasWarn bool) {
	s.ChannelMessageDelete(m.ChannelID, m.ID)

	var spoiler string
	spoiler = strings.Replace(m.Content, "!sp ", "", 1)
	spoiler = strings.Replace(spoiler, "!cw ", "", 1)

	var warning string
	var content string

	if hasWarn {
		split := strings.SplitN(spoiler, "|", 2)

		warning = split[0]
		content = strings.TrimSpace(split[1])
	} else {
		warning = "Content Warning"
		content = spoiler
	}

	rotated := shuffle(content)
	if len(rotated) > 200 {
		rotated = fmt.Sprintf("%s ...", rotated[0:200])
	}

	markdown := fmt.Sprintf("### %s \n\nPosted by `%s` \n\n%s", warning, m.Author.Username, content)
	markdownURL := pasteURL(warning, markdown)

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    m.Author.Username,
			IconURL: m.Author.AvatarURL(""),
		},
		Color:       c.EmbedColor,
		Title:       warning,
		Description: rotated,
		URL:         markdownURL,
	})
}

func cmdHelp(s *discordgo.Session, m *discordgo.MessageCreate, c *config) {
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    c.BotName,
			IconURL: s.State.User.AvatarURL("128"),
			URL:     fmt.Sprintf("https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=92160", c.ClientID),
		},
		Color: c.EmbedColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Usage",
				Value:  "```!sp your message```",
				Inline: false,
			},
			{
				Name:   "Custom warning message",
				Value:  "```!sp spoiler for star warps | your message```",
				Inline: false,
			},
		},
	})
}

func cmdStats(s *discordgo.Session, m *discordgo.MessageCreate, c *config) {
	stats := runtime.MemStats{}
	runtime.ReadMemStats(&stats)

	users := 0
	for _, guild := range s.State.Ready.Guilds {
		users += len(guild.Members)
	}
	channels := 0
	for _, guild := range s.State.Ready.Guilds {
		channels += len(guild.Channels)
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    c.BotName,
			IconURL: s.State.User.AvatarURL("128"),
			URL:     fmt.Sprintf("https://discordapp.com/oauth2/authorize?client_id=%s&scope=bot&permissions=92160", c.ClientID),
		},
		Color: c.EmbedColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Discordgo",
				Value:  discordgo.VERSION,
				Inline: true,
			},
			{
				Name:   "Go",
				Value:  runtime.Version(),
				Inline: true,
			},
			{
				Name:   "Memory",
				Value:  fmt.Sprintf("%s / %s", humanize.Bytes(stats.Alloc), humanize.Bytes(stats.Sys)),
				Inline: true,
			},
			{
				Name:   "Tasks",
				Value:  fmt.Sprintf("%d", runtime.NumGoroutine()),
				Inline: true,
			},
			{
				Name:   "Servers",
				Value:  fmt.Sprintf("%d", len(s.State.Ready.Guilds)),
				Inline: true,
			},
			{
				Name:   "Channels",
				Value:  fmt.Sprintf("%d", channels),
				Inline: true,
			},
			{
				Name:   "Users",
				Value:  fmt.Sprintf("%d", users),
				Inline: true,
			},
		},
	})
}

func shuffleChar(c rune) string {
	if c == ' ' {
		return " "
	}

	boxes := []string{"█", "▓", "▒", "░", "█", "▓", "▒", "░", "█", "▓", "▒", "░", "<", ">", "/"}

	n := rand.Intn(len(boxes))
	return boxes[n]
}

func shuffle(s string) string {
	var shuffled []string
	for _, rune := range s {
		shuffled = append(shuffled, shuffleChar(rune))
	}

	return strings.Join(shuffled, "")
}
