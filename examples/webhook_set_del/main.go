package main

import (
	"log"

	imbotapi "github.com/imbot-io/imbot-sdk-go"
)

func main() {
	botToken := "APITOKEN"
	bot, err := imbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	wh, _ := imbotapi.NewWebhook("https://www.example.com/" + botToken)

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal(err)
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("get webhook successfully,result: %v", info)

	bot.Request(imbotapi.DeleteWebhookConfig{})

	info, err = bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("get webhook successfully,result: %v", info)
}
