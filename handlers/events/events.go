/*
Package events handles searching for Eventful events and announcing matching results in the area.
*/
package events

import (
	"time"

	"fmt"
	"strconv"

	"os"
	"strings"

	"github.com/sha1sum/eventful"
	"github.com/sha1sum/golang_groupme_bot/bot"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
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

type eventSearch struct {
	Term          string    `bson:"term"`
	Users         []user    `bson:"users"`
	LatestCreated time.Time `bson:"latest_created"`
}

type user struct {
	UserID string `bson:"user_id"`
}

// DB is the name of the MongoDB database
var DB string

// Handle takes a search term and queries the Eventful API for matching results in the given ZIP code
func (handler Handler) Handle(term string, c chan []*bot.OutgoingMessage, message bot.IncomingMessage) {
	if message.SenderType == "bot" {
		return
	}
	if len(term) < 4 {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "You must provide a search term at least 4 characters in length."}}
		return
	}
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
	res, err := client.SearchEvents(term, dateString, zip, radius, sort, 10, 1)
	if err != nil {
		c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Err: err}}
		return
	}
	if len(res.Events) < 1 {
		c <- []*bot.OutgoingMessage{
			&bot.OutgoingMessage{
				Text: "No events found for \"" + term + "\" in the next " + strconv.Itoa(days) + "days.",
			},
			handler.trackEvent(term, message),
		}
		return
	}
	if res.TotalItems > 10 {
		res, err = client.SearchEvents(term, dateString, zip, 25, sort, 10, 1)
		if err != nil {
			c <- []*bot.OutgoingMessage{&bot.OutgoingMessage{Err: err}}
			return
		}
	}
	messages := outputEvents(res.Events)
	messages = append(messages, handler.trackEvent(term, message))
	c <- messages
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

func (handler Handler) trackEvent(term string, message bot.IncomingMessage) *bot.OutgoingMessage {
	uri := os.Getenv("MONGOLAB_URI")
	if uri == "" {
		return &bot.OutgoingMessage{Text: "no connection string provided"}
	}
	DB = os.Getenv("MONGOLAB_DB")
	if uri == "" {
		return &bot.OutgoingMessage{Text: "no database provided"}
	}
	sess, err := mgo.Dial(uri)
	if err != nil {
		return &bot.OutgoingMessage{Text: "can't connect to mongo"}
	}
	defer sess.Close()
	col := sess.DB(DB).C("groupmeEventSearchesV1")
	u := user{UserID: message.UserID}
	var td eventSearch
	col.Find(bson.M{"term": strings.ToLower(term)}).One(&td)
	if len(td.Term) < 1 {
		loc, _ := time.LoadLocation("America/New_York")
		col.Insert(eventSearch{Term: strings.ToLower(term), LatestCreated: time.Date(1970, 1, 1, 0, 0, 0, 0, loc), Users: []user{u}})
	} else {
		col.Update(bson.M{"term": strings.ToLower(term)}, bson.M{"$addToSet": bson.M{"users": u}})
	}
	/* col.Upsert(bson.M{
		"term":strings.ToLower(term),
	}, bson.M{
		"latest_created": time.Now(),
		"$addToSet": bson.M{
			"users": u,
		},
	}) */
	return &bot.OutgoingMessage{Text: "New events for \"" + strings.ToLower(term) + "\" will now be tracked."}
}

func (handler Handler) SetupSearch(botID string) {
	key := handler.Key
	if len(key) < 1 {
		fmt.Println("Key is empty.")
		return
	}
	zip := handler.ZIP
	if len(zip) != 5 {
		fmt.Println("ZIP is empty")
		return
	}
	radius := handler.Radius
	if radius == 0 {
		radius = 100
	}
	ticker := time.NewTicker(10 * time.Minute)
	quit := make(chan struct{})

	go func(handler Handler, botID string) {
		c := make(chan *bot.OutgoingMessage)
		go monitorForMessages(c, botID)
		uri := os.Getenv("MONGOLAB_URI")
		if uri == "" {
			fmt.Println("no connection string provided")
			os.Exit(1)
		}
		DB = os.Getenv("MONGOLAB_DB")
		if uri == "" {
			fmt.Println("no database provided")
			os.Exit(1)
		}
		sess, err := mgo.Dial(uri)
		if err != nil {
			fmt.Printf("Can't connect to mongo, go error %v\n", err)
			os.Exit(1)
		}
		col := sess.DB(DB).C("groupmeEventSearchesV1")
		var searches []eventSearch
		err = col.Find(nil).All(&searches)
		if err != nil {
			fmt.Println(err)
		}
		key := handler.Key
		client := eventful.New(key)
		for _, v := range searches {
			handler.recurringSearch(col, v, c, client)
		}
		sess.Close()
		for {
			select {
			case <-ticker.C:
				sess, err := mgo.Dial(uri)
				if err != nil {
					fmt.Printf("Can't connect to mongo, go error %v\n", err)
					os.Exit(1)
				}
				col := sess.DB(DB).C("groupmeEventSearchesV1")
				var searches []eventSearch
				err = col.Find(nil).All(&searches)
				if err != nil {
					fmt.Println(err)
				}
				key := handler.Key
				client := eventful.New(key)
				for _, v := range searches {
					handler.recurringSearch(col, v, c, client)
				}
				sess.Close()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}(handler, botID)
}

func monitorForMessages(c chan *bot.OutgoingMessage, botID string) {
	for {
		select {
		case m := <-c:
			fmt.Println("Message received...")
			if m.Err != nil {
				_, err := bot.PostMessage(&bot.OutgoingMessage{Text: fmt.Sprint(m.Err)}, botID)
				if err != nil {
					fmt.Println(err)
				}
				return
			}
			_, err := bot.PostMessage(m, botID)
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("Outgoing message: %+v", m)
			time.Sleep(time.Second)
		}
	}
}

func (handler Handler) recurringSearch(col *mgo.Collection, search eventSearch, c chan *bot.OutgoingMessage, client *eventful.Client) {
	zip := handler.ZIP
	if len(zip) != 5 {
		return
	}
	radius := handler.Radius
	if radius == 0 {
		radius = 100
	}
	sort := handler.SortOrder
	if len(sort) < 1 {
		sort = "date"
	}
	start := time.Now()
	end := start.AddDate(0, 0, 180)
	dateString := fmt.Sprintf("%d%02d%02d00-%d%02d%02d00",
		start.Year(), start.Month(), start.Day(), end.Year(), end.Month(), end.Day())
	events, err := client.SearchAllEvents(search.Term, dateString, zip, radius, sort, 10)
	if err != nil {
		return
	}
	var event eventful.Event
	latest := search.LatestCreated
	outtf := "1/2/2006 3:04pm"
	found := false
	var created time.Time
	intf := "2006-01-02 15:04:05"
	for _, v := range events {
		created, _ = time.Parse(intf, v.Created)
		if created.After(latest) {
			latest = created
			found = true
			event = v
		} else {
		}
	}
	if found {
		fmt.Println("Found an event!")
		startTime, _ := time.Parse(intf, event.StartTime)
		text := fmt.Sprintf("%s: %s [at %s in %s] %s",
			startTime.Format(outtf),
			event.Title,
			event.VenueName,
			event.CityName,
			event.URL,
		)
		loci := make([][2]int, len(search.Users))
		mentions := make([]int, len(search.Users))
		for i, v := range search.Users {
			mentions[i], _ = strconv.Atoi(v.UserID)
			loci[i] = [2]int{
				len(startTime.Format(outtf)) + 2,
				len(event.Title),
			}
		}
		search.LatestCreated = latest
		col.Update(bson.M{"term": search.Term}, search)
		c <- &bot.OutgoingMessage{
			Text: text,
			Attachments: []bot.Attachment{
				bot.Attachment{
					Loci:    loci,
					Type:    "mentions",
					UserIDs: mentions,
				},
			},
		}
	}
}
