# Inline Keyboard

This bot waits for you to send it the message "open" before sending you an
inline keyboard containing a URL and some numbers. When a number is clicked, it
sends you a message with your selected number.

```go
package main

import (
	"log"
	"os"

	"github.com/imbot-io/imbot-sdk-go"
)

var numericKeyboard = imbotapi.NewInlineKeyboardMarkup(
	imbotapi.NewInlineKeyboardRow(
		imbotapi.NewInlineKeyboardButtonURL("1.com", "http://1.com"),
		imbotapi.NewInlineKeyboardButtonData("2", "2"),
		imbotapi.NewInlineKeyboardButtonData("3", "3"),
	),
	imbotapi.NewInlineKeyboardRow(
		imbotapi.NewInlineKeyboardButtonData("4", "4"),
		imbotapi.NewInlineKeyboardButtonData("5", "5"),
		imbotapi.NewInlineKeyboardButtonData("6", "6"),
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

	// Loop through each update.
	for update := range updates {
		// Check if we've gotten a message update.
		if update.Message != nil {
			// Construct a new message from the given chat ID and containing
			// the text that we received.
			msg := imbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)

			// If the message was open, add a copy of our numeric keyboard.
			switch update.Message.Text {
			case "open":
				msg.ReplyMarkup = numericKeyboard

			}

			// Send the message.
			if _, err = bot.Send(msg); err != nil {
				panic(err)
			}
		} else if update.CallbackQuery != nil {
			// Respond to the callback query, telling DE-IM to show the user
			// a message with the data received.
			callback := imbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
			if _, err := bot.Request(callback); err != nil {
				panic(err)
			}

			// And finally, send a message containing the data received.
			msg := imbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Data)
			if _, err := bot.Send(msg); err != nil {
				panic(err)
			}
		}
	}
}
```
