package main

import (
	"log"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

func main() {
	botToken := "APITOKEN"
	bot, err := agentsdk.NewAgentAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	msg := agentsdk.NewPhoto("test002", agentsdk.FileURL("https://s1.xx.io/apps/ak-banner.png"))
	res, err := bot.Request(msg)
	if err != nil {
		log.Fatal("Unable to send text message")
	}
	log.Printf("send photo message successfully,result: %s", string(res.Result))
}
