package main

import (
	"fmt"
	"github.com/nicklaw5/helix"
	log "github.com/sirupsen/logrus"
	"sync"
	"time"
)

// TenseiTwitch ...
type TenseiTwitch struct {
	helix *helix.Client

	TwitchStreamers     []*TwitchStreamer
	TwitchStreamerMutex sync.RWMutex

	RateLimit          int
	RateLimitRemaining int
	RateLimitReset     time.Time
	RateLimitMutex     sync.RWMutex
}

// NewTwitch creates twitch client
func (tb *TenseiBot) NewTwitch() {
	var err error
	tb.Twitch.helix, err = helix.NewClient(&helix.Options{
		ClientID: tb.Config.Twitch.ClientID,
	})
	if err != nil {
		log.Fatalf("[TWITCH] failed creating client: %v", err)
	}

	tb.Twitch.TwitchStreamers = tb.GetStreamers()
	go tb.startTwitchJobs()

	log.Info("[MODULE] twitch loaded")
}

// GetUsers ...
func (tt *TenseiTwitch) GetUsers(ids []string, logins []string) ([]helix.User, error) {
	resp, err := tt.helix.GetUsers(&helix.UsersParams{
		IDs:    ids,
		Logins: logins,
	})
	if err != nil {
		return nil, fmt.Errorf("[TWITCH] failed getting users: %v", err)
	}

	tt.updateRateLimit(resp.GetRateLimit(), resp.GetRateLimitRemaining(), resp.GetRateLimitReset())

	return resp.Data.Users, nil
}

func (tt *TenseiTwitch) getStream(ids []string, logins []string) ([]helix.Stream, error) {
	resp, err := tt.helix.GetStreams(&helix.StreamsParams{
		UserIDs:    ids,
		UserLogins: logins,
	})
	if err != nil {
		return nil, fmt.Errorf("[TWITCH] failed checking if user is live ids: %s, logins: %s err: %v", ids, logins, err)
	}

	defer tt.updateRateLimit(resp.GetRateLimit(), resp.GetRateLimitRemaining(), resp.GetRateLimitReset())
	return resp.Data.Streams, nil
}

func (tt *TenseiTwitch) getGameByID(id string) (*helix.Game, error) {
	resp, err := tt.helix.GetGames(&helix.GamesParams{
		IDs: []string{id},
	})
	if err != nil || len(resp.Data.Games) < 1 {
		return nil, fmt.Errorf("[TWITCH] failed getting game for id: %s, err: %v", id, err)
	}

	defer tt.updateRateLimit(resp.GetRateLimit(), resp.GetRateLimitRemaining(), resp.GetRateLimitReset())
	return &resp.Data.Games[0], nil
}

func isStreaming(stream *helix.Stream) bool {
	return stream != nil && stream.ID != "" && stream.Type == "live"
}

// ignore ratelimits for now
func (tt *TenseiTwitch) updateRateLimit(limit, limitRemaining, limitReset int) {
	tt.RateLimitMutex.Lock()
	defer tt.RateLimitMutex.Unlock()

	reset := time.Unix(int64(limitReset), 0)
	tt.RateLimit = limit
	tt.RateLimitRemaining = limitRemaining
	tt.RateLimitReset = reset
	log.Debugf("[TWITCH_RATELIMIT] ratelimit: %d, remaining: %d, reset: %s", limit, limitRemaining, reset.Sub(time.Now()))
}

func (tb *TenseiBot) startTwitchJobs() {
	log.Infof("[TWITCH] starting %d TWITCH_JOBS", len(tb.Twitch.TwitchStreamers))
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		var wg sync.WaitGroup
		wg.Add(len(tb.Twitch.TwitchStreamers))
		tb.Twitch.TwitchStreamerMutex.Lock()
		for _, streamer := range tb.Twitch.TwitchStreamers {
			go tb.NewTwitchJob(streamer, &wg)
		}
		wg.Wait()
		tb.Twitch.TwitchStreamerMutex.Unlock()
	}
}
