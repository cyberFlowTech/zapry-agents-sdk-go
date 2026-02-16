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

	msg := imbotapi.NewMessage("test001", "hello world")
	res, err := bot.Request(msg)
	if err != nil {
		log.Fatal("Unable to send text message")
	}
	log.Printf("send text message successfully,result: %s", string(res.Result))
}
