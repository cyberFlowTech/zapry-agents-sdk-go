# Golang bindings for the De-IM Bot API

[![Go Reference](https://pkg.go.dev/badge/github.com/imbot-io/imbot-sdk-go.svg)](https://pkg.go.dev/github.com/imbot-io/imbot-sdk-go)
[![Test](https://github.com/imbot-io/imbot-sdk-go/actions/workflows/test.yml/badge.svg)](https://github.com/imbot-io/imbot-sdk-go/actions/workflows/test.yml)

All methods are fairly self-explanatory, and reading the [godoc](https://pkg.go.dev/github.com/imbot-io/imbot-sdk-go) page should
explain everything. If something isn't clear, open an issue or submit
a pull request.

There are more tutorials and high-level information on the website, [go-de-im-bot-api.dev](https://go-de-im-bot-api.dev).

The scope of this project is just to provide a wrapper around the API
without any additional features. There are other projects for creating
something with plugins and command handlers without having to design
all that yourself.

Join [the development group](https://de-im.io/go_de-im_bot_api) if
you want to ask questions or discuss development.

## Example

First, ensure the library is installed and up to date by running
`go get -u github.com/imbot-io/imbot-sdk-go`.

This is a very simple bot that just displays any gotten updates,
then replies it to that chat.

```go
package main

import (
	"log"

	"github.com/imbot-io/imbot-sdk-go"
)

func main() {
	bot, err := imbotapi.NewBotAPI("MyAwesomeBotToken")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := imbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil { // If we got a message
			log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

			msg := imbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
			msg.ReplyToMessageID = update.Message.MessageID

			bot.Send(msg)
		}
	}
}
```


