package main

import (
	"log"
	"net/http"

	imbotapi "github.com/imbot-io/imbot-sdk-go"
)

var numericKeyboard = imbotapi.NewReplyKeyboard(
	imbotapi.NewKeyboardButtonRow(
		imbotapi.NewKeyboardButton("Regular Button 1"),
		imbotapi.NewKeyboardButton("Regular Button 2"),
		imbotapi.NewKeyboardButton("Regular Button 3"),
	),
	imbotapi.NewKeyboardButtonRow(
		imbotapi.NewKeyboardButton("Regular Button 4"),
		imbotapi.NewKeyboardButton("Regular Button 5"),
		imbotapi.NewKeyboardButton("Regular Button 6"),
	),
)

var inlineNumericKeyboard = imbotapi.NewInlineKeyboardMarkup(
	imbotapi.NewInlineKeyboardRow(
		imbotapi.NewInlineKeyboardButtonURL("1.com", "http://1.com"),
		imbotapi.NewInlineKeyboardButtonData("Inline Button 2", "20"),
		imbotapi.NewInlineKeyboardButtonData("Inline Button 3", "30"),
	),
	imbotapi.NewInlineKeyboardRow(
		imbotapi.NewInlineKeyboardButtonData("Inline Button 4", "40"),
		imbotapi.NewInlineKeyboardButtonData("Inline Button 5", "50"),
		imbotapi.NewInlineKeyboardButtonData("Inline Button 6", "60"),
	),
)

func main() {
	botToken := "APITOKEN"
	bot, err := imbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	wh, _ := imbotapi.NewWebhook("http://xxx.com/" + botToken)

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
		msg := imbotapi.NewMessage(update.Message.Chat.ID, "")

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
