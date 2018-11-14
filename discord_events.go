package main

import (
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

// GuildCreate handles guild join events
func (tb *TenseiBot) GuildCreate(s *discordgo.Session, m *discordgo.GuildCreate) {
	// add guild to db
	owner, _ := s.User(m.OwnerID)
	log.Infof("[JOIN] guild: %s(%s), owner: %s(%s), member_count: %d", m.Name, m.ID, owner.String(), m.OwnerID, m.MemberCount)
	tb.AddGuildToDB(m.Guild, owner)
}

// GuildMemberAdd handles guild member join events
func (tb *TenseiBot) GuildMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	// add guild to db
	guild, _ := s.Guild(m.GuildID)
	joined, _ := m.JoinedAt.Parse()
	log.Infof("[MEMBER_JOIN] guild: %s(%s), member: %s(%s), account_created: %s, account_age: %s", guild.Name, guild.ID, m.User.String(), m.User.ID, m.JoinedAt, time.Since(joined))
}

// GuildMemberRemove handles guild member remove events
func (tb *TenseiBot) GuildMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	// add guild to db
	guild, _ := s.Guild(m.GuildID)
	log.Infof("[MEMBER_REMOVE] guild: %s(%s), member: %s(%s)", guild.Name, guild.ID, m.User.String(), m.User.ID)
}

// GuildMemberUpdate handles guild member update events
func (tb *TenseiBot) GuildMemberUpdate(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	// add guild to db
	guild, _ := s.Guild(m.GuildID)
	log.Infof("[MEMBER_UPDATE] guild: %s(%s), member: %s(%s)", guild.Name, guild.ID, m.User.String(), m.User.ID)
}

// MessageDelete handles message delete events
func (tb *TenseiBot) MessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	// add guild to db
	guild, _ := s.Guild(m.GuildID)
	tb.Discord.msgCacheMutex.Lock()
	defer tb.Discord.msgCacheMutex.Unlock()

	for _, mc := range tb.Discord.msgCache {
		if m.ID != mc.ID {
			continue
		}
		log.Infof("[MESSAGE_DELETE] guild: %s(%s), member: %s(%s), attachments: %d, message: %s", guild.Name, guild.ID, mc.Author.String(), mc.Author.ID, len(mc.Attachments), mc.Content)
		return
	}
	log.Infof("[MESSAGE_DELETE] guild: %s(%s), message out of cache, rip", guild.Name, guild.ID)
}
