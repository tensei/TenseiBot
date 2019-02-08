package main

import (
	"github.com/BurntSushi/toml"
	log "github.com/sirupsen/logrus"
)

// TenseiConfig ...
type TenseiConfig struct {
	Google struct {
		APIKey string `toml:"api_key"`
	} `toml:"google"`
	Discord struct {
		Prefix  string `toml:"prefix"`
		Token   string `toml:"token"`
		OwnerID string `toml:"owner_id"`
	} `toml:"discord"`
	Twitch struct {
		ClientID string `toml:"client_id"`
	} `toml:"twitch"`
	Database struct {
		Dialect          string `toml:"dialect"`
		ConnectionString string `toml:"connection_string"`
	} `toml:"database"`
}

// Load loads config file
func (tc *TenseiConfig) Load(file string) {
	_, err := toml.DecodeFile(file, &tc)
	if err != nil {
		log.Fatalf("[CONFIG] failed decoding file %s: %v", file, err)
	}
}
