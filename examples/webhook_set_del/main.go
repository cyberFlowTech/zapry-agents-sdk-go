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

	wh, _ := agentsdk.NewWebhook("https://www.example.com/" + botToken)

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal(err)
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("get webhook successfully,result: %v", info)

	bot.Request(agentsdk.DeleteWebhookConfig{})

	info, err = bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("get webhook successfully,result: %v", info)
}
