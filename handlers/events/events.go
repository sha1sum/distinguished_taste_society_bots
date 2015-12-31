/*
Package events handles searching for Eventful events and announcing matching results in the area.
*/
package events

import (
	"time"

	"fmt"
	"strconv"

	"github.com/sha1sum/eventful"
	"github.com/sha1sum/golang_groupme_bot/bot"
)

// Handler will satisfy the bot.Handler interface.
type Handler struct {
	Key string
	ZIP string
	// Radius is in miles
	Radius int
	// Days is the number of days in the future to search
	Days int
	// SortOrder is the field on which to sort events
	SortOrder string
}

// Handle takes a search term and queries the Eventful API for matching results in the given ZIP code
func (handler Handler) Handle(term string, c chan []*bot.OutgoingMessage, message bot.IncomingMessage) {
	if message.SenderType == "bot" { return }
	key := handler.Key
	if len(key) < 1 {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "Events API key is not yet set."}}
		return
	}
	zip := handler.ZIP
	if len(zip) != 5 {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "ZIP code for event search is not yet set."}}
		return
	}
	radius := handler.Radius
	if radius == 0 {
		radius = 100
	}
	days := handler.Days
	if days == 0 {
		days = 30
	}
	sort := handler.SortOrder
	if len(sort) < 1 {
		sort = "date"
	}
	client := eventful.New(key)
	start := time.Now()
	end := start.AddDate(0, 0, days)
	dateString := fmt.Sprintf("%d%02d%02d00-%d%02d%02d00",
		start.Year(), start.Month(), start.Day(), end.Year(), end.Month(), end.Day())
	res, err := client.SearchEvents(term, dateString, zip, radius, sort)
	if err != nil {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Err: err}}
		return
	}
	if len(res.Events) < 1 {
		c <- []*bot.OutgoingMessage{
			&bot.OutgoingMessage{
				Text: "No events found for \"" + term + "\" in the next " + strconv.Itoa(days) + "days.",
			},
		}
		return
	}
	if res.TotalItems > 10 {
		res, err = client.SearchEvents(term, dateString, zip, 25, sort)
		if err != nil {
			c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Err: err}}
			return
		}
	}
	c <- outputEvents(res.Events)
	c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "Test"}}
}

func outputEvents(events []eventful.Event) []*bot.OutgoingMessage {
	em := make([]*bot.OutgoingMessage, 0)
	intf := "2006-01-02 15:04:05"
	outtf := "1/2/2006 3:04pm"
	for _, v := range events {
		startTime, _ := time.Parse(intf, v.StartTime)
		text := fmt.Sprintf("%s: %s [at %s in %s] %s",
			startTime.Format(outtf),
			v.Title,
			v.VenueName,
			v.CityName,
			v.URL,
		)
		em = append(em, &bot.OutgoingMessage{Text: text})
	}
	return em
}

func (handler Handler) SetupSearch(c chan []*bot.OutgoingMessage, duration int) {
	key := handler.Key
	if len(key) < 1 {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "Events API key is not yet set."}}
		return
	}
	zip := handler.ZIP
	if len(zip) != 5 {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "ZIP code for event search is not yet set."}}
		return
	}
	radius := handler.Radius
	if radius == 0 {
		radius = 100
	}
}
