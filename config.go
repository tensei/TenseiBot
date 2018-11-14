package main

import (
	"encoding/json"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
)

// TenseiConfig ...
type TenseiConfig struct {
	Google struct {
		APIKey string `json:"apiKey"`
	} `json:"google"`
	Discord struct {
		Prefix  string `json:"prefix"`
		Token   string `json:"token"`
		OwnerID string `json:"ownerID"`
	} `json:"discord"`
	Twitch struct {
		ClientID string `json:"clientID"`
	} `json:"twitch"`
	Database struct {
		Dialect          string `json:"dialect"`
		ConnectionString string `json:"connectionString"`
	} `json:"database"`
}

// Load loads config file
func (tc *TenseiConfig) Load(file string) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalf("[CONFIG] failed reading file %s: %v", file, err)
	}
	err = json.Unmarshal(b, &tc)
	if err != nil {
		log.Fatalf("[CONFIG] failed unmarshaling file %s: %v", file, err)
	}
}
