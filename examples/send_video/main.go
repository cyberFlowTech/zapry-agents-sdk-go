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

	msg := agentsdk.NewVideo("test002", agentsdk.FileURL("https://s1.xx.io/apps/deletefromspace.mp4"))
	res, err := bot.Request(msg)
	if err != nil {
		log.Fatal("Unable to send text message")
	}
	log.Printf("send video message successfully,result: %s", string(res.Result))
}
