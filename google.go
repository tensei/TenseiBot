package main

import (
	"context"

	"cloud.google.com/go/translate"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
)

// TenseiGoogle google part of the bot
type TenseiGoogle struct {
	ctx                context.Context
	ctxCancelFunc      context.CancelFunc
	TranslateClient    *translate.Client
	supportedLanguages []translate.Language
}

// NewGoogle creates all google clients
func (tb *TenseiBot) NewGoogle() {
	tb.Google.NewTranslateClientWithKey(tb.Config.Google.APIKey)
	tb.Google.ctx, tb.Google.ctxCancelFunc = context.WithCancel(context.Background())
}

// NewTranslateClientWithKey ...
func (tg *TenseiGoogle) NewTranslateClientWithKey(apiKey string) {
	client, err := translate.NewClient(tg.ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("[GOOGLE] failed creating translate client: %v", err)
	}
	tg.TranslateClient = client
	tg.supportedLanguages, _ = tg.TranslateClient.SupportedLanguages(tg.ctx, language.English)
	log.Info("[MODULE] translate loaded")
}

// Translate translates text to the target language
func (tg *TenseiGoogle) Translate(text, targetLanguage string) (string, error) {
	lang, err := language.Parse(targetLanguage)
	if err != nil {
		return "", err
	}
	resp, err := tg.TranslateClient.Translate(tg.ctx, []string{text}, lang, nil)
	if err != nil {
		return "", err
	}
	return resp[0].Text, nil
}
