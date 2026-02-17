package main

import (
	"log"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go/imbotapi"
)

func main() {
	botToken := "API_TOKEN"
	bot, err := agentsdk.NewAgentAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	setCommands := agentsdk.NewSetMyCommands(agentsdk.BotCommand{
		Command:     "test",
		Description: "a test command",
	})

	res, err := bot.Request(setCommands)
	if err != nil {
		log.Fatal("Unable to set commands")
	}
	log.Printf("set command successfully,result: %s", string(res.Result))

	commands, err := bot.GetMyCommands()
	if err != nil {
		log.Fatal("Unable to get commands")
	}

	log.Printf("get command successfully,result: %v", commands)

	if len(commands) != 1 {
		log.Fatal("Incorrect number of commands returned")
	}

	if commands[0].Command != "test" || commands[0].Description != "a test command" {
		log.Fatal("Commands were incorrectly set")
	}

	setCommands = agentsdk.NewSetMyCommandsWithScope(agentsdk.NewBotCommandScopeAllPrivateChats(), agentsdk.BotCommand{
		Command:     "private",
		Description: "a private command",
	})

	if _, err := bot.Request(setCommands); err != nil {
		log.Fatal("Unable to set commands")
	}

	commands, err = bot.GetMyCommandsWithConfig(agentsdk.NewGetMyCommandsWithScope(agentsdk.NewBotCommandScopeAllPrivateChats()))
	if err != nil {
		log.Fatal("Unable to get commands")
	}

	if len(commands) != 1 {
		log.Fatal("Incorrect number of commands returned")
	}

	if commands[0].Command != "private" || commands[0].Description != "a private command" {
		log.Fatal("Commands were incorrectly set")
	}

}
