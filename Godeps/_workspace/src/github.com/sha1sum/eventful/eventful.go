/*
  PUBLIC DOMAIN STATEMENT
  To the extent possible under law, Ian Davis has waived all copyright
  and related or neighboring rights to this Source Code file.
  This work is published from the United Kingdom.
*/

// Client for the Eventful API
package eventful

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type Client struct {
	APIKey string
}

type SearchEventsResponse struct {
	TotalItems int     `json:"total_items,string"`
	PageNumber int     `json:"page_number,string"`
	PageSize   int     `json:"page_size,string"`
	PageCount  int     `json:"page_count,string"`
	Events     []Event `json:"events"`
}

type RawSearchEventsResponse struct {
	TotalItems int            `json:"total_items,string"`
	PageNumber int            `json:"page_number,string"`
	PageSize   int            `json:"page_size,string"`
	PageCount  int            `json:"page_count,string"`
	Events     EventContainer `json:"events"`
}

type EventContainer struct {
	Events []Event `json:"event"`
}

type Event struct {
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	ID          string     `json:"id"`
	VenueID     string     `json:"venue_id"`
	VenueName   string     `json:"venue_name"`
	VenueURL    string     `json:"venue_url"`
	StartTime   string     `json:"start_time""`
	StopTime    string     `json:"stop_time""`
	AllDay      string     `json:"all_day""`
	Latitude    string     `json:"latitude""`
	Longitude   string     `json:"longitude""`
	CityName    string     `json:"city_name""`
	Description string     `json:"description""`
	Created string `json:"created"`
	Image       *ImageInfo `json:"image""`
}

type ImageInfo struct {
	Image
	Small  *Image `json:"small"`
	Medium *Image `json:"medium"`
	Thumb  *Image `json:"thumb"`
}

type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width,string"`
	Height int    `json:"height,string"`
}

func New(apikey string) *Client {
	return &Client{APIKey: apikey}
}

func (client *Client) SearchEvents(srch string, date string, location string, within int, sort string) (*SearchEventsResponse, error) {
	var clean SearchEventsResponse
	var data RawSearchEventsResponse

	if len(date) < 1 {
		date = "&date=" + date
	} else {
		date = ""
	}
	url := fmt.Sprintf("http://api.eventful.com/json/events/search?app_key=%s%s&keywords=%s&location=%s&within=%s&sort_order=%s", url.QueryEscape(client.APIKey), url.QueryEscape(date), url.QueryEscape(srch), url.QueryEscape(location), strconv.Itoa(within), sort)
	println(url)
	resp, err := http.Get(url)

	if err != nil {
		return nil, fmt.Errorf("SearchEvents failed with http error: %s", err.Error())
	}

	defer resp.Body.Close()

	dec := json.NewDecoder(resp.Body)

	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("SearchEvents failed to parse JSON response: %s", err.Error())
	}

	if err != nil {
		return nil, err
	}

	clean.TotalItems = data.TotalItems
	clean.PageNumber = data.PageNumber
	clean.PageSize = data.PageSize
	clean.PageCount = data.PageCount
	clean.Events = data.Events.Events

	return &clean, nil
}
