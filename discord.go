package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

type commandFunc func(s *discordgo.Session, m *discordgo.MessageCreate, command string)

// TenseiDiscord discord part of the bot
type TenseiDiscord struct {
	c      *discordgo.Session
	prefix string

	msgCacheMutex sync.Mutex
	msgCache      []*discordgo.Message
	msgCacheLimit int

	commands map[string]command

	cooldowns      map[string]*Cooldowns
	CooldownsMutex sync.Mutex
}

type command struct {
	f   commandFunc
	cds map[string]time.Time
}

// Cooldowns struct for storing channel specific cooldowns
type Cooldowns struct {
	Translate time.Time
	Twitch    time.Time
}

// SetupDiscordCommands ...
func (tb *TenseiBot) SetupDiscordCommands() {
	tb.Discord.commands = map[string]command{
		"!tr":     command{f: discordTranslate(tb), cds: make(map[string]time.Time)},
		"!twitch": command{f: discordTwitch(tb), cds: make(map[string]time.Time)},
		"!uptime": command{f: discordUptime(tb), cds: make(map[string]time.Time)},
		"!stats":  command{f: discordStats(tb), cds: make(map[string]time.Time)},
	}
}

// NewDiscord creates a new discord session
func (tb *TenseiBot) NewDiscord() {
	token := tb.Config.Discord.Token
	if token == "" {
		log.Fatal("[DISCORD] missing TOKEN in config file")
	}

	s, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatalf("[DISCORD] failed creating new session: %v", err)
	}

	tb.Discord.msgCacheLimit = 1000
	tb.Discord.msgCache = []*discordgo.Message{}
	tb.Discord.cooldowns = make(map[string]*Cooldowns)

	s.AddHandler(tb.CommandHandler)
	s.AddHandler(tb.GuildCreate)
	s.AddHandler(tb.GuildMemberAdd)
	s.AddHandler(tb.GuildMemberRemove)
	s.AddHandler(tb.GuildMemberUpdate)
	s.AddHandler(tb.MessageDelete)

	tb.SetupDiscordCommands()

	err = s.Open()
	if err != nil {
		log.Fatalf("failed opening connection to discord: %v", err)
	}

	tb.Discord.c = s
	tb.Discord.prefix = tb.Config.Discord.Prefix
	log.Info("[MODULE] discord loaded")
}

// CommandHandler ...
func (tb *TenseiBot) CommandHandler(s *discordgo.Session, m *discordgo.MessageCreate) {
	tb.Discord.addMessageToCache(m)

	if !strings.HasPrefix(m.Content, tb.Discord.prefix) {
		return
	}

	parts := strings.SplitN(m.Content, " ", 2)
	for k, c := range tb.Discord.commands {
		if strings.EqualFold(k, parts[0]) {
			guild, _ := s.Guild(m.GuildID)
			log.Infof("[COMMAND] %s used in server: %s(%s), user: %s(%s) ", parts[0], guild.Name, guild.ID, m.Author.String(), m.Author.ID)
			go c.f(s, m, k)
			return
		}
	}
}

func (td *TenseiDiscord) getCooldown(c string, ch string) time.Time {
	td.CooldownsMutex.Lock()
	defer td.CooldownsMutex.Unlock()

	if cds, ok := td.commands[c]; ok {
		if chcd, ok := cds.cds[ch]; ok {
			return chcd
		}
	}
	td.commands[c].cds[ch] = time.Now().Add(-1 * time.Hour)
	return time.Now().Add(-1 * time.Hour)
}

func (td *TenseiDiscord) setCooldown(c, channelID string, time time.Time) {
	td.CooldownsMutex.Lock()
	defer td.CooldownsMutex.Unlock()

	td.commands[c].cds[channelID] = time
}

func (td *TenseiDiscord) addMessageToCache(m *discordgo.MessageCreate) {
	td.msgCacheMutex.Lock()
	defer td.msgCacheMutex.Unlock()

	if len(td.msgCache) >= td.msgCacheLimit {
		td.msgCache = td.msgCache[1:]
	}
	td.msgCache = append(td.msgCache, m.Message)
}

func (tb *TenseiBot) isOwner(id string) bool {
	return tb.Config.Discord.OwnerID == id
}

func (tb *TenseiBot) isDiscordCommandOnCD(cmnd, chID, gID, aID string, t int64) bool {
	if tb.isOwner(aID) {
		return false
	}
	cd := tb.Discord.getCooldown(cmnd, chID)
	if time.Now().After(cd) {
		tb.Discord.setCooldown(cmnd, chID, time.Now().Add(time.Duration(t)*time.Second))
		return false
	}
	return true
}

func discordTranslate(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		set := tb.GetGuildSettingsFromDB(m.GuildID)
		if tb.isDiscordCommandOnCD(command, m.ChannelID, m.GuildID, m.Author.ID, *set.TranslateCooldown) {
			log.Debugf("[COMMAND] %s is on cd", command)
			return
		}

		parts := strings.SplitN(m.Content, " ", 3)
		if len(parts) < 3 {
			log.Info("[TRANSLATE] failed translating missing text")
			return
		}

		text := strings.TrimSpace(parts[2])
		if text == "" {
			log.Info("[TRANSLATE] failed translating text is empty")
		}

		target := strings.TrimSpace(parts[1])

		// if user didn't use the right language format get it from the supported list
		if len(target) > 2 && len(tb.Google.supportedLanguages) > 0 {
			for _, l := range tb.Google.supportedLanguages {
				if strings.EqualFold(target, l.Name) {
					target = l.Tag.String()
					break
				}
			}
		}

		output, err := tb.Google.Translate(text, target)
		if err != nil {
			log.Infof("[TRANSLATE] failed translating '%s', error: %v", text, err)
			return
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Fields: []*discordgo.MessageEmbedField{
				&discordgo.MessageEmbedField{
					Name:   "Input",
					Value:  text,
					Inline: false,
				},
				&discordgo.MessageEmbedField{
					Name:   "Output",
					Value:  output,
					Inline: false,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: " - Google Cloud Translate",
			},
		})
	}
}

func discordUptime(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		if m.Author.ID != tb.Config.Discord.OwnerID {
			return
		}
		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Fields: []*discordgo.MessageEmbedField{
				&discordgo.MessageEmbedField{
					Name:   "Started",
					Value:  tb.started.Format(time.Stamp),
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Uptime",
					Value:  fmt.Sprintf("%s", time.Since(tb.started)),
					Inline: true,
				},
			},
		})
	}
}

func discordStats(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		if m.Author.ID != tb.Config.Discord.OwnerID {
			return
		}
		guilds := len(s.State.Guilds)
		users := 0
		for _, g := range s.State.Guilds {
			users += len(g.Members)
		}

		s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title: "Stats",
			Fields: []*discordgo.MessageEmbedField{
				&discordgo.MessageEmbedField{
					Name:   "Guilds",
					Value:  fmt.Sprintf("%d", guilds),
					Inline: true,
				},
				&discordgo.MessageEmbedField{
					Name:   "Users",
					Value:  fmt.Sprintf("%d", users),
					Inline: true,
				},
			},
		})
	}
}

func discordTwitch(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		set := tb.GetGuildSettingsFromDB(m.GuildID)
		parts := strings.SplitN(m.Content, " ", 3)
		switch parts[1] {
		case "id":
			// get user id
			if tb.isDiscordCommandOnCD(command, m.ChannelID, m.GuildID, m.Author.ID, *set.TwitchCooldown) {
				log.Debugf("[COMMAND] %s is on cd", command)
				return
			}
			tb.Twitch.discordGetUserTwitchID(s, m, parts[2])
		case "name":
			// get user name
			if tb.isDiscordCommandOnCD(command, m.ChannelID, m.GuildID, m.Author.ID, *set.TwitchCooldown) {
				log.Debugf("[COMMAND] %s is on cd", command)
				return
			}
			tb.Twitch.discordGetUserTwitchName(s, m, parts[2])
		case "add":
			// add stuff
		case "reomove":
			// remove stuff
		}
	}
}

func (tt *TenseiTwitch) discordGetUserTwitchID(s *discordgo.Session, m *discordgo.MessageCreate, name string) {
	users := tt.GetUsers(nil, []string{name})
	if len(users) < 1 {
		return
	}
	user := users[0]
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title: user.DisplayName,
		URL:   fmt.Sprintf("https://twitch.tv/%s", user.DisplayName),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: user.ProfileImageURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:  "ID",
				Value: user.ID,
			},
		},
	})
}

func (tt *TenseiTwitch) discordGetUserTwitchName(s *discordgo.Session, m *discordgo.MessageCreate, id string) {
	users := tt.GetUsers([]string{id}, nil)
	if len(users) < 1 {
		return
	}
	user := users[0]
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title: user.ID,
		URL:   fmt.Sprintf("https://twitch.tv/%s", user.DisplayName),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: user.ProfileImageURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			&discordgo.MessageEmbedField{
				Name:  "Name",
				Value: user.DisplayName,
			},
		},
	})
}
