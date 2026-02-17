package main

import (
	"log"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go/imbotapi"
)

func main() {
	botToken := "APITOKEN"
	bot, err := agentsdk.NewAgentAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	msg := agentsdk.NewMessage("test001", "hello world")
	res, err := bot.Request(msg)
	if err != nil {
		log.Fatal("Unable to send text message")
	}
	log.Printf("send text message successfully,result: %s", string(res.Result))
}
