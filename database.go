package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mssql"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	log "github.com/sirupsen/logrus"
)

// Guild struct for database
type Guild struct {
	ID        string `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time

	OwnerName string
	OwnerID   string

	AdminRoleID string

	TranslateCooldown *int64 `gorm:"default:3"`
	TwitchCooldown    *int64 `gorm:"default:3"`
}

// TwitchStreamer stores data about a streamer
type TwitchStreamer struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Name            string
	ChannelID       string
	ProfileImageURL string

	StreamStartTime time.Time
	StreamEndTime   time.Time

	TwitchAlertSubscriptions []*TwitchAlertSubscription
}

// TwitchAlertSubscription ...
type TwitchAlertSubscription struct {
	ID        uint `gorm:"primary_key"`
	CreatedAt time.Time
	UpdatedAt time.Time

	MessageID string
	ChannelID string
	GuildID   string

	TwitchStreamerID uint
}

// NewDatabase create/opens a database
func (tb *TenseiBot) NewDatabase() {
	db := tb.Config.Database.ConnectionString
	dialect := tb.Config.Database.Dialect

	if db == "" || dialect == "" {
		log.Fatal("[DATABASE] missing connectionString/dialect in config file")
	}

	var err error
	tb.db, err = gorm.Open(dialect, db)
	if err != nil {
		log.Fatalf("[DATABASE] failed opening %s: %v", db, err)
	}

	tb.db.AutoMigrate(&Guild{})
	tb.db.AutoMigrate(&TwitchStreamer{})
	tb.db.AutoMigrate(&TwitchAlertSubscription{})

	log.Info("[MODULE] database loaded")
}

// AddGuildToDB add new guild to database
func (tb *TenseiBot) AddGuildToDB(g *discordgo.Guild, owner *discordgo.User) {
	var dbg Guild
	tb.db.Where("id = ?", g.ID).First(&dbg)
	if dbg.ID == g.ID {
		return
	}
	dbg.ID = g.ID
	dbg.OwnerID = owner.ID
	dbg.OwnerName = owner.String()
	tb.db.Create(&dbg)
}

// ChangeGuildTranslateCooldown change TranslateCooldown for guild
func (tb *TenseiBot) ChangeGuildTranslateCooldown(newCd int64, g *discordgo.Guild) {
	var dbg Guild
	tb.db.Where("id = ?", g.ID).First(&dbg)
	dbg.TranslateCooldown = &newCd
	tb.db.Save(&dbg)
}

// GetGuildSettingsFromDB change TranslateCooldown for guild
func (tb *TenseiBot) GetGuildSettingsFromDB(id string) Guild {
	var dbg Guild
	tb.db.Where("id = ?", id).First(&dbg)
	return dbg
}

// UpdateGuildSettings change TranslateCooldown for guild
func (tb *TenseiBot) UpdateGuildSettings(g Guild) {
	tb.db.Save(&g)
}

// AddStreamer adds a new streamer to the database
func (tb *TenseiBot) AddStreamer(streamer *TwitchStreamer) {
	tb.db.Create(streamer)
}

// GetStreamers returns a list of all TwitchStreamers in the database
func (tb *TenseiBot) GetStreamers() []*TwitchStreamer {
	var streamers []*TwitchStreamer
	tb.db.Set("gorm:auto_preload", true).Find(&streamers)
	return streamers
}

// UpdateStreamer updates streamer in database
func (tb *TenseiBot) UpdateStreamer(streamer *TwitchStreamer) {
	tb.db.Save(streamer)
}

// GetStreamerWithID returns the streamer given the id
func (tb *TenseiBot) GetStreamerWithID(id string) (*TwitchStreamer, error) {
	var streamer TwitchStreamer
	tb.db.Set("gorm:auto_preload", true).Where("channel_id = ?", id).First(&streamer)
	if !strings.EqualFold(streamer.ChannelID, id) {
		return nil, fmt.Errorf("[DATABASE] no streamer with id: %s found", id)
	}
	return &streamer, nil
}

// GetStreamerWithName return the streamer given the name
func (tb *TenseiBot) GetStreamerWithName(name string) (*TwitchStreamer, error) {
	var streamer TwitchStreamer
	tb.db.Set("gorm:auto_preload", true).Where("name = ?", strings.ToLower(name)).First(&streamer)
	if !strings.EqualFold(streamer.Name, name) {
		return nil, fmt.Errorf("[DATABASE] no streamer with name: %s found", name)
	}
	return &streamer, nil
}
