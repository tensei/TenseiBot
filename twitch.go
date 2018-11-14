package main

import (
	"github.com/nicklaw5/helix"
	log "github.com/sirupsen/logrus"
)

// TenseiTwitch ...
type TenseiTwitch struct {
	helix *helix.Client
}

// NewTwitch creates twitch client
func (tb *TenseiBot) NewTwitch() {
	client, err := helix.NewClient(&helix.Options{
		ClientID: tb.Config.Twitch.ClientID,
	})
	if err != nil {
		log.Fatalf("[TWITCH] failed creating client: %v", err)
	}
	tb.Twitch.helix = client

	log.Info("[MODULE] twitch loaded")
}

// GetUsers ...
func (tt *TenseiTwitch) GetUsers(ids []string, logins []string) []helix.User {
	resp, err := tt.helix.GetUsers(&helix.UsersParams{
		IDs:    ids,
		Logins: logins,
	})
	if err != nil {
		log.Warnf("[TWITCH] failed getting users: %v", err)
		return nil
	}
	return resp.Data.Users
}
