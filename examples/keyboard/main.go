package main

import (
	"log"
	"net/http"

	agentsdk "github.com/cyberFlowTech/zapry-agents-sdk-go"
)

var numericKeyboard = agentsdk.NewReplyKeyboard(
	agentsdk.NewKeyboardButtonRow(
		agentsdk.NewKeyboardButton("Regular Button 1"),
		agentsdk.NewKeyboardButton("Regular Button 2"),
		agentsdk.NewKeyboardButton("Regular Button 3"),
	),
	agentsdk.NewKeyboardButtonRow(
		agentsdk.NewKeyboardButton("Regular Button 4"),
		agentsdk.NewKeyboardButton("Regular Button 5"),
		agentsdk.NewKeyboardButton("Regular Button 6"),
	),
)

var inlineNumericKeyboard = agentsdk.NewInlineKeyboardMarkup(
	agentsdk.NewInlineKeyboardRow(
		agentsdk.NewInlineKeyboardButtonURL("1.com", "http://1.com"),
		agentsdk.NewInlineKeyboardButtonData("Inline Button 2", "20"),
		agentsdk.NewInlineKeyboardButtonData("Inline Button 3", "30"),
	),
	agentsdk.NewInlineKeyboardRow(
		agentsdk.NewInlineKeyboardButtonData("Inline Button 4", "40"),
		agentsdk.NewInlineKeyboardButtonData("Inline Button 5", "50"),
		agentsdk.NewInlineKeyboardButtonData("Inline Button 6", "60"),
	),
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
			msg.Text = "I understand /sayhi  /status /inlineButton /keyboardButton ."
		case "sayhi":
			msg.Text = "Hi :)"
		case "status":
			msg.Text = "I'm ok."
		case "inlineButton":
			msg.Text = "Inline keyboard example"
			msg.ReplyMarkup = inlineNumericKeyboard
		case "keyboardButton":
			msg.Text = "Regular keyboard example"
			msg.ReplyMarkup = numericKeyboard
		default:
			msg.Text = "I don't know that command"
		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
		log.Println("msg send successfully, msg:", msg.Text)
	}
}
