package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/nicklaw5/helix"
	log "github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
)

// TwitchJob ...
func (tb *TenseiBot) NewTwitchJob(streamer *TwitchStreamer, wg *sync.WaitGroup) {
	defer wg.Done()
	var stream *helix.Stream
	streams, err := tb.Twitch.getStream([]string{streamer.ChannelID}, nil)
	if err != nil {
		log.Warn(err)
		return
	}
	if len(streams) > 0 {
		stream = &streams[0]
	}

	// wait 3minutes
	if time.Now().UTC().Sub(streamer.StreamEndTime) < time.Duration(time.Minute*3) {
		return
	}

	// stream is offline
	if !isStreaming(stream) && streamer.StreamLength() >= 0 {
		return
	}

	// stream started
	if isStreaming(stream) && streamer.StreamLength() >= 0 {
		log.Infof("[TWITCH_JOB] streamer %s started streaming %s", streamer.Name, stream.StartedAt.UTC().Format("15:04:05 MST"))
		streamer.StreamStartTime = stream.StartedAt.UTC()
		users, _ := tb.Twitch.GetUsers([]string{streamer.ChannelID}, nil)
		if len(users) > 0 {
			streamer.ProfileImageURL = users[0].ProfileImageURL
		}
		embed := tb.Twitch.createLiveEmbed(stream, streamer)
		for _, alerts := range streamer.TwitchAlertSubscriptions {
			msg, err := tb.Discord.c.ChannelMessageSendEmbed(alerts.ChannelID, embed)
			if err != nil {
				log.Errorf("[TWITCH_JOB] (stream start) failed sending embed to channel: %s, streamer: %s", alerts.ChannelID, streamer.Name)
			} else {
				alerts.MessageID = msg.ID
			}
		}
		tb.UpdateStreamer(streamer)
		return
	}

	// update the embed
	if isStreaming(stream) && streamer.StreamLength() <= 0 {
		// update embed
		log.Debugf("[TWITCH_JOB] updating embeds for streamer %s", streamer.Name)
		embed := tb.Twitch.createLiveEmbed(stream, streamer)
		for _, alerts := range streamer.TwitchAlertSubscriptions {
			if alerts.MessageID == "" {
				msg, err := tb.Discord.c.ChannelMessageSendEmbed(alerts.ChannelID, embed)
				if err != nil {
					log.Errorf("[TWITCH_JOB] (stream update) failed sending embed to channel: %s, streamer: %s", alerts.ChannelID, streamer.Name)
				} else {
					alerts.MessageID = msg.ID
					tb.UpdateStreamer(streamer)
				}
			} else {
				_, err = tb.Discord.c.ChannelMessageEditEmbed(alerts.ChannelID, alerts.MessageID, embed)
				if err != nil {
					log.Errorf("[TWITCH_JOB] (stream update) failed editing embed in channel: %s, streamer: %s", alerts.ChannelID, streamer.Name)
				}
			}
		}
	}

	// stream went offline
	if !isStreaming(stream) && streamer.StreamLength() <= 0 {
		streamer.StreamEndTime = time.Now().UTC().Add(-(time.Minute * 3))
		log.Infof("[TWITCH_JOB] streamer %s stopped streaming length %s", streamer.Name, streamer.StreamLength())
		embed := createEndEmbed(streamer)
		for _, alerts := range streamer.TwitchAlertSubscriptions {
			_, err := tb.Discord.c.Channel(alerts.ChannelID)
			if err != nil {
				log.Warnf("[TWITCH_JOB] error getting channel: %s")
			}
			_, err = tb.Discord.c.ChannelMessageEditEmbed(alerts.ChannelID, alerts.MessageID, embed)
			if err != nil {
				log.Errorf("[TWITCH_JOB] (stream end) failed editing embed in channel: %s, streamer: %s", alerts.ChannelID, streamer.Name)
			}
		}
		tb.UpdateStreamer(streamer)
	}
}

// StreamLength return how long a stream was online
func (s *TwitchStreamer) StreamLength() time.Duration {
	return s.StreamEndTime.Sub(s.StreamStartTime)
}

func (tt *TenseiTwitch) createLiveEmbed(stream *helix.Stream, streamer *TwitchStreamer) *discordgo.MessageEmbed {
	channelURL := fmt.Sprintf("https://twitch.tv/%s", streamer.Name)
	thumbnailURL := fmt.Sprintf("https://static-cdn.jtvnw.net/previews-ttv/live_user_%s-1920x1080.jpg?t=%s", streamer.Name, time.Now().Unix())
	liveFor := time.Now().UTC().Sub(streamer.StreamStartTime)

	gameName := "???"
	game, err := tt.getGameByID(stream.GameID)
	if err == nil {
		gameName = game.Name
	}

	return &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("%s", strings.Title(streamer.Name)),
			URL:  channelURL,
		},
		Title: stream.Title,
		URL:   channelURL,
		Image: &discordgo.MessageEmbedImage{
			URL: thumbnailURL,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Category",
				Value:  gameName,
				Inline: true,
			},
			{
				Name:   "Viewers",
				Value:  fmt.Sprintf("%d", stream.ViewerCount),
				Inline: true,
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: streamer.ProfileImageURL,
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Live for %s", humanizeDuration(liveFor)),
		},
		Color: 0xFF0000,
	}
}

func createEndEmbed(streamer *TwitchStreamer) *discordgo.MessageEmbed {
	channelURL := fmt.Sprintf("https://twitch.tv/%s", streamer.Name)

	endTime := streamer.StreamEndTime.UTC().Format(time.RFC822)
	startTime := streamer.StreamStartTime.UTC().Format(time.RFC822)

	if streamer.ChannelID == "11249217" {
		loc, _ := time.LoadLocation("Asia/Tokyo")
		jaFormat := "2006/02/01 15:04:05 MS"
		endTime = streamer.StreamEndTime.In(loc).Format(jaFormat)
		startTime = streamer.StreamStartTime.In(loc).Format(jaFormat)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("**Started at:** %s\n", startTime))
	sb.WriteString(fmt.Sprintf("__**Ended at:** %s__\n", endTime))
	sb.WriteString(fmt.Sprintf("**Total Time:** %s", humanizeDuration(streamer.StreamLength())))

	return &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name: fmt.Sprintf("%s was Live", strings.Title(streamer.Name)),
			URL:  channelURL,
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: streamer.ProfileImageURL,
		},
		URL:         channelURL,
		Description: sb.String(),
	}
}
