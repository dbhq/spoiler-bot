# spoiler-bot

Replaces a text post with a warning message and link to read the orignal message

### Building

```
go get -u github.com/BurntSushi/toml github.com/bwmarrin/discordgo github.com/dustin/go-humanize
go build spoilerbot.go
```

### Usage

Spoiler warning

`!sp some message` or `!sp Custom Warning | some message`

Help

`!help`
