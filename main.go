/*
Distinguished Taste Society Bots are a set of bots used by the Distinguished Taste Society on their GroupMe groups. The
bots are built to use github.com/sha1sum/golang_groupme_bot.
*/

package main

import (
	"os"

	"github.com/sha1sum/distinguished_taste_society_bots/handlers/adultpoints"
	"github.com/sha1sum/distinguished_taste_society_bots/handlers/events"
	"github.com/sha1sum/distinguished_taste_society_bots/handlers/googlenews"
	"github.com/sha1sum/golang_groupme_bot/bot"
)

func main() {
	commands := make([]bot.Command, 0)

	// Google News search bot
	news := bot.Command{
		Triggers: []string{
			"!news",
			// Having the spaced version helps for those with mobile keyboards that automatically insert spaces after
			// exclamation points
			"! news",
		},
		Handler: new(googlenews.Handler),
		BotID:   os.Getenv("GROUPME_BOT_ID"),
	}

	// Adult Point tracking bot
	adult := bot.Command{
		Triggers: []string{
			"!adultme",
			"! adultme",
			"!award",
			"! award",
			"!reject",
			"! reject",
			"!adults",
			"! adults",
		},
		Handler: new(adultpoints.Handler),
		BotID:   os.Getenv("GROUPME_BOT_ID"),
	}

	// Event Search bot
	eventsHandler := events.Handler{Key: os.Getenv("EVENTFUL_API_KEY"), ZIP: "33701"}
	eventBot := bot.Command{
		Triggers: []string{
			"!events",
			"! events",
		},
		Handler: eventsHandler,
		BotID:   os.Getenv("GROUPME_BOT_ID"),
	}

	commands = append(commands, news)
	commands = append(commands, adult)
	commands = append(commands, eventBot)

	eventsHandler.SetupSearch(eventBot.BotID)

	bot.Listen(commands)
}
