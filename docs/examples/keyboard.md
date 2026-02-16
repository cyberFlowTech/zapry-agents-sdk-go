# Keyboard

This bot shows a numeric keyboard when you send a "open" message and hides it
when you send "close" message.

```go
package main

import (
	"log"
	"os"

	"github.com/imbot-io/imbot-sdk-go"
)

var numericKeyboard = imbotapi.NewReplyKeyboard(
	imbotapi.NewKeyboardButtonRow(
		imbotapi.NewKeyboardButton("1"),
		imbotapi.NewKeyboardButton("2"),
		imbotapi.NewKeyboardButton("3"),
	),
	imbotapi.NewKeyboardButtonRow(
		imbotapi.NewKeyboardButton("4"),
		imbotapi.NewKeyboardButton("5"),
		imbotapi.NewKeyboardButton("6"),
	),
)

func main() {
	bot, err := imbotapi.NewBotAPI(os.Getenv("DE_IM_APITOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := imbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore non-Message updates
			continue
		}

		msg := imbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)

		switch update.Message.Text {
		case "open":
			msg.ReplyMarkup = numericKeyboard
		case "close":
			msg.ReplyMarkup = imbotapi.NewRemoveKeyboard(true)
		}

		if _, err := bot.Send(msg); err != nil {
			log.Panic(err)
		}
	}
}
```
