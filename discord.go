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
func (tb *TenseiBot) SetupDiscordCommands(prefix string) {
	tb.Discord.commands = map[string]command{
		prefix + "tr":     {f: discordTranslate(tb), cds: make(map[string]time.Time)},
		prefix + "twitch": {f: discordTwitch(tb), cds: make(map[string]time.Time)},
		prefix + "uptime": {f: discordUptime(tb), cds: make(map[string]time.Time)},
		prefix + "stats":  {f: discordStats(tb), cds: make(map[string]time.Time)},
		prefix + "tb":     {f: discordTenseiBot(tb), cds: make(map[string]time.Time)},
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
	tb.Discord.prefix = tb.Config.Discord.Prefix
	tb.Discord.c = s

	s.AddHandler(tb.CommandHandler)
	s.AddHandler(tb.GuildCreate)
	s.AddHandler(tb.GuildMemberAdd)
	s.AddHandler(tb.GuildMemberRemove)
	s.AddHandler(tb.GuildMemberUpdate)
	s.AddHandler(tb.MessageDelete)

	tb.SetupDiscordCommands(tb.Config.Discord.Prefix)

	err = s.Open()
	if err != nil {
		log.Fatalf("failed opening connection to discord: %v", err)
	}

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

func isServerAdmin(guild Guild, member *discordgo.Member) bool {
	if guild.OwnerID == member.User.ID {
		return true
	}
	return contains(member.Roles, guild.AdminRoleID)
}

func (tb *TenseiBot) isDiscordCommandOnCD(cmd, chID string, member *discordgo.Member, t int64, guildSetting Guild) bool {
	if tb.isOwner(member.User.ID) {
		return false
	}
	// don't check cooldown when member has the admin role
	if isServerAdmin(guildSetting, member) {
		return false
	}

	cd := tb.Discord.getCooldown(cmd, chID)
	if time.Now().After(cd) {
		tb.Discord.setCooldown(cmd, chID, time.Now().Add(time.Duration(t)*time.Second))
		return false
	}
	return true
}

func discordTranslate(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		set := tb.GetGuildSettingsFromDB(m.GuildID)
		member, _ := s.GuildMember(m.GuildID, m.Author.ID)
		if tb.isDiscordCommandOnCD(command, m.ChannelID, member, *set.TranslateCooldown, set) {
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

		// if member didn't use the right language format get it from the supported list
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

		_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Input",
					Value:  text,
					Inline: false,
				},
				{
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
		_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Started",
					Value:  tb.started.Format(time.Stamp),
					Inline: true,
				},
				{
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
		if !tb.isOwner(m.Author.ID) {
			return
		}
		guilds := len(s.State.Guilds)
		users := 0
		for _, g := range s.State.Guilds {
			users += len(g.Members)
		}

		_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
			Title: "Stats",
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Guilds",
					Value:  fmt.Sprintf("%d", guilds),
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
}

func discordTwitch(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		set := tb.GetGuildSettingsFromDB(m.GuildID)
		member, _ := s.GuildMember(m.GuildID, m.Author.ID)
		if tb.isDiscordCommandOnCD(command, m.ChannelID, member, *set.TwitchCooldown, set) {
			return
		}
		parts := strings.Split(m.Content, " ")
		if len(parts) < 3 {
			return
		}
		switch parts[1] {
		case "id":
			// get member id
			if tb.isDiscordCommandOnCD(command, m.ChannelID, member, *set.TwitchCooldown, set) {
				log.Debugf("[COMMAND] %s is on cd", command)
				return
			}
			tb.Twitch.discordGetUserTwitchID(s, m, parts[2])
		case "name":
			// get member name
			if tb.isDiscordCommandOnCD(command, m.ChannelID, member, *set.TwitchCooldown, set) {
				log.Debugf("[COMMAND] %s is on cd", command)
				return
			}
			tb.Twitch.discordGetUserTwitchName(s, m, parts[2])
		case "add":
			// add stuff
			if !isServerAdmin(set, member) {
				return
			}
			streamer, err := tb.GetStreamerWithName(parts[2])
			if err != nil {
				log.Warn(err)
				users, err := tb.Twitch.GetUsers(nil, []string{parts[2]})
				if err != nil {
					log.Warn(err)
					return
				}
				user := users[0]
				streamer = &TwitchStreamer{
					Name:            strings.ToLower(user.DisplayName),
					ChannelID:       user.ID,
					ProfileImageURL: user.ProfileImageURL,
				}
				tb.AddStreamer(streamer)
				tb.Twitch.RateLimitMutex.Lock()
				tb.Twitch.TwitchStreamers = append(tb.Twitch.TwitchStreamers, streamer)
				tb.Twitch.RateLimitMutex.Unlock()
			}
			if !strings.HasPrefix(parts[3], "<#") {
				return
			}
			channelID := parts[3][2 : len(parts[3])-1]
			if hasAlertSubscription(streamer, channelID) {
				return
			}
			channel, err := s.Channel(channelID)
			if err != nil {
				DiscordSendErrorMessageEmbed(s, m.ChannelID, "couldn't find channel with id: %s, err: %v", channelID, err)
				return
			}
			// make sure the channel is on the same server
			if channel.GuildID == m.GuildID {
				streamer.TwitchAlertSubscriptions = append(streamer.TwitchAlertSubscriptions, &TwitchAlertSubscription{
					ChannelID: channelID,
					GuildID:   m.GuildID,
				})
				tb.UpdateStreamer(streamer)
				DiscordSendSuccessMessageEmbed(s, m.ChannelID, "added %s alert to channel %s", streamer.Name, channel.Mention())
			} else {
				DiscordSendErrorMessageEmbed(s, m.ChannelID, "can't add channel on other server")
			}
		case "remove":
			// remove stuff
			if !isServerAdmin(set, member) {
				return
			}
		case "online":
			streamer := parts[2]
			streams, err := tb.Twitch.getStream(nil, []string{streamer})
			if err != nil {
				log.Warn(err)
				return
			}
			if len(streams) > 0 {
				DiscordSendSuccessMessageEmbed(s, m.ChannelID, fmt.Sprintf("%s stream is currently %v", streamer, isStreaming(&streams[0])))
			} else {
				DiscordSendErrorMessageEmbed(s, m.ChannelID, fmt.Sprintf("%s stream is currently offline", streamer))
			}
		}
	}
}

func (tt *TenseiTwitch) discordGetUserTwitchID(s *discordgo.Session, m *discordgo.MessageCreate, name string) {
	users, err := tt.GetUsers(nil, []string{name})
	if err != nil {
		log.Warn(err)
	}
	if len(users) < 1 {
		return
	}
	user := users[0]
	_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title: user.DisplayName,
		URL:   fmt.Sprintf("https://twitch.tv/%s", user.DisplayName),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: user.ProfileImageURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "ID",
				Value: user.ID,
			},
		},
	})
}

func (tt *TenseiTwitch) discordGetUserTwitchName(s *discordgo.Session, m *discordgo.MessageCreate, id string) {
	users, err := tt.GetUsers([]string{id}, nil)
	if err != nil {
		log.Warn(err)
		return
	}
	if len(users) < 1 {
		return
	}
	user := users[0]
	_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title: user.ID,
		URL:   fmt.Sprintf("https://twitch.tv/%s", user.DisplayName),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: user.ProfileImageURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Name",
				Value: user.DisplayName,
			},
		},
	})
}

func discordTenseiBot(tb *TenseiBot) func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
	return func(s *discordgo.Session, m *discordgo.MessageCreate, command string) {
		guild, _ := s.Guild(m.GuildID)
		if m.Author.ID != guild.OwnerID && !tb.isOwner(m.Author.ID) {
			return
		}
		set := tb.GetGuildSettingsFromDB(m.GuildID)
		args := strings.Split(m.Content, " ")[1:]
		switch args[0] {
		case "set":
			switch args[1] {
			case "adminrole":
				_, _ = s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
					Description: fmt.Sprintf("updating admin_role_id from '%s' to '%s'", set.AdminRoleID, args[2]),
				})
				set.AdminRoleID = args[2]
				tb.UpdateGuildSettings(set)
				return
			}
		}
	}
}
