package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

func contains(slice []string, s string) bool {
	for _, value := range slice {
		if strings.EqualFold(value, s) {
			return true
		}
	}
	return false
}

func hasAlertSubscription(streamer *TwitchStreamer, channelID string) bool {
	for _, alert := range streamer.TwitchAlertSubscriptions {
		if alert.ChannelID == channelID {
			return true
		}
	}
	return false
}

func humanizeDuration(duration time.Duration) string {
	var sb strings.Builder

	hours := int(duration.Hours())
	if hours == 1 {
		sb.WriteString(fmt.Sprintf("%d hour, ", hours))
	}
	if hours > 1 {
		sb.WriteString(fmt.Sprintf("%d hours, ", hours))
	}

	minutes := int(duration.Minutes()) - (hours * 60)
	if minutes == 1 {
		sb.WriteString(fmt.Sprintf("%d minute", minutes))
	}

	if minutes > 1 {
		sb.WriteString(fmt.Sprintf("%d minutes", minutes))
	}

	return sb.String()
}

// DiscordSendSuccessMessageEmbed ...
func DiscordSendSuccessMessageEmbed(s *discordgo.Session, channelID, message string, args ...interface{}) {
	_, err := s.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Description: fmt.Sprintf(message, args...),
		Color:       0x00ff00,
	})
	if err != nil {
		log.Errorf("[DISCORD] error sending success message to channel %s, err: %v", channelID, err)
	}
}

// DiscordSendSuccessMessageEmbed ...
func DiscordSendErrorMessageEmbed(s *discordgo.Session, channelID, message string, args ...interface{}) {
	_, err := s.ChannelMessageSendEmbed(channelID, &discordgo.MessageEmbed{
		Description: fmt.Sprintf(message, args...),
		Color:       0xff0000,
	})
	if err != nil {
		log.Errorf("[DISCORD] error sending error message to channel %s, err: %v", channelID, err)
	}
}
