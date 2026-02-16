package main

import (
	"log"

	imbotapi "github.com/imbot-io/imbot-sdk-go"
)

func main() {
	botToken := "APITOKEN"
	bot, err := imbotapi.NewAgentAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	msg := imbotapi.NewPhoto("test002", imbotapi.FileURL("https://s1.xx.io/apps/ak-banner.png"))
	res, err := bot.Request(msg)
	if err != nil {
		log.Fatal("Unable to send text message")
	}
	log.Printf("send photo message successfully,result: %s", string(res.Result))
}
