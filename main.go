package main

import (
	"os"
	"os/signal"
	"time"

	"github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
)

// TenseiBot ...
type TenseiBot struct {
	db      *gorm.DB
	Discord *TenseiDiscord
	Google  *TenseiGoogle
	Twitch  *TenseiTwitch
	Config  *TenseiConfig

	started time.Time
}

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	// log.SetLevel(log.DebugLevel)
}

func main() {
	tb := NewTenseiBot()

	defer tb.Close()
	tb.Config.Load("config.json")

	tb.NewDatabase()
	tb.NewDiscord()
	tb.NewGoogle()
	tb.NewTwitch()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c
}

// NewTenseiBot ...
func NewTenseiBot() *TenseiBot {
	return &TenseiBot{
		Discord: new(TenseiDiscord),
		Google:  new(TenseiGoogle),
		Twitch:  new(TenseiTwitch),
		Config:  new(TenseiConfig),
		started: time.Now(),
	}
}

// Close closing everything
func (tb *TenseiBot) Close() {
	_ = tb.db.Close()
	_ = tb.Discord.c.Close()
	tb.Google.ctxCancelFunc()
	_ = tb.Google.TranslateClient.Close()
}
