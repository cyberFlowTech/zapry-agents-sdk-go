# Getting Started

This library is designed as a simple wrapper around the de-im Bot API.
It's encouraged to read [de-im's docs][de-im-docs] first to get an
understanding of what Bots are capable of doing. They also provide some good
approaches to solve common problems.

[de-im-docs]: https://core.de-im.io/bots

## Installing

```bash
go get -u github.com/imbot-io/imbot-sdk-go
```

## A Simple Bot

To walk through the basics, let's create a simple echo bot that replies to your
messages repeating what you said. Make sure you get an API token from
[@Botfather][botfather] before continuing.

Let's start by constructing a new [BotAPI][bot-api-docs].

[botfather]: https://t.me/Botfather
[bot-api-docs]: https://pkg.go.dev/github.com/imbot-io/imbot-sdk-go?tab=doc#BotAPI

```go
package main

import (
	"os"

	"github.com/imbot-io/imbot-sdk-go"
)

func main() {
	bot, err := imbotapi.NewBotAPI(os.Getenv("DE_IM_APITOKEN"))
	if err != nil {
		panic(err)
	}

	bot.Debug = true
}
```

Instead of typing the API token directly into the file, we're using
environment variables. This makes it easy to configure our Bot to use the right
account and prevents us from leaking our real token into the world. Anyone with
your token can send and receive messages from your Bot!

We've also set `bot.Debug = true` in order to get more information about the
requests being sent to DE-IM. If you run the example above, you'll see
information about a request to the [`getMe`][get-me] endpoint. The library
automatically calls this to ensure your token is working as expected. It also
fills in the `Self` field in your `BotAPI` struct with information about the
Bot.

Now that we've connected to De-IM, let's start getting updates and doing
things. We can add this code in right after the line enabling debug mode.

[get-me]: https://core.de-im.io/bots/api#getme

```go
	// Create a new UpdateConfig struct with an offset of 0. Offsets are used
	// to make sure De-IM knows we've handled previous values and we don't
	// need them repeated.
	updateConfig := imbotapi.NewUpdate(0)

	// Tell De-IM we should wait up to 30 seconds on each request for an
	// update. This way we can get information just as quickly as making many
	// frequent requests without having to send nearly as many.
	updateConfig.Timeout = 30

	// Start polling De-IM for updates.
	updates := bot.GetUpdatesChan(updateConfig)

	// Let's go through each update that we're getting from De-IM.
	for update := range updates {
		// De-IM can send many types of updates depending on what your Bot
		// is up to. We only want to look at messages for now, so we can
		// discard any other updates.
		if update.Message == nil {
			continue
		}

		// Now that we know we've gotten a new message, we can construct a
		// reply! We'll take the Chat ID and Text from the incoming message
		// and use it to create a new message.
		msg := imbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		// We'll also say that this message is a reply to the previous message.
		// For any other specifications than Chat ID or Text, you'll need to
		// set fields on the `MessageConfig`.
		msg.ReplyToMessageID = update.Message.MessageID

		// Okay, we're sending our message off! We don't care about the message
		// we just sent, so we'll discard it.
		if _, err := bot.Send(msg); err != nil {
			// Note that panics are a bad way to handle errors. De-IM can
			// have service outages or network errors, you should retry sending
			// messages or more gracefully handle failures.
			panic(err)
		}
	}
```

Congratulations! You've made your very own bot!

Now that you've got some of the basics down, we can start talking about how the
library is structured and more advanced features.
