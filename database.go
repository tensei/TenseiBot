package main

import (
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
