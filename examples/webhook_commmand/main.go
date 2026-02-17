package main

import (
	"log"
	"net/http"

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

	wh, _ := agentsdk.NewWebhook("http://xxx.com/" + botToken)

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal(err)
	}

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Fatal(err)
	}

	if info.LastErrorDate != 0 {
		log.Printf("de-im callback failed: %s", info.LastErrorMessage)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)
	go http.ListenAndServe("0.0.0.0:29339", nil)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			log.Println("message is nil")
			continue
		}

		if !update.Message.IsCommand() { // ignore any non-command Messages
			log.Println("message is not command, update:", update)
			continue
		}

		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		msg := agentsdk.NewMessage(update.Message.Chat.ID, "")

		// Extract the command from the Message.
		switch update.Message.Command() {
		case "help":
			msg.Text = "I understand /sayhi and /status."
		case "sayhi":
			msg.Text = "Hi :)"
		case "status":
			msg.Text = "I'm ok."
		default:
			msg.Text = "I don't know that command"
		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
		log.Println("msg send successfully, msg:", msg.Text)
	}
}
