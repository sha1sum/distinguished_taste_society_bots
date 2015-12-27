package matchers

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
)

type (
	// Item defines the fields associated with the item tag
	// in the rss document.
	Item struct {
		XMLName     xml.Name `xml:"item"`
		PubDate     string   `xml:"pubDate"`
		Title       string   `xml:"title"`
		Description string   `xml:"description"`
		Link        string   `xml:"link"`
		GUID        string   `xml:"guid"`
		GeoRssPoint string   `xml:"georss:point"`
	}

	// image defines the fields associated with the image tag
	// in the rss document.
	image struct {
		XMLName xml.Name `xml:"image"`
		URL     string   `xml:"url"`
		Title   string   `xml:"title"`
		Link    string   `xml:"link"`
	}

	// channel defines the fields associated with the channel tag
	// in the rss document.
	channel struct {
		XMLName        xml.Name `xml:"channel"`
		Title          string   `xml:"title"`
		Description    string   `xml:"description"`
		Link           string   `xml:"link"`
		PubDate        string   `xml:"pubDate"`
		LastBuildDate  string   `xml:"lastBuildDate"`
		TTL            string   `xml:"ttl"`
		Language       string   `xml:"language"`
		ManagingEditor string   `xml:"managingEditor"`
		WebMaster      string   `xml:"webMaster"`
		Image          image    `xml:"image"`
		Item           []Item   `xml:"item"`
	}

	// RSSDocument defines the fields associated with the rss document.
	RSSDocument struct {
		XMLName xml.Name `xml:"rss"`
		Channel channel  `xml:"channel"`
	}
)

// Retrieve performs a HTTP Get request for the rss feed and decodes the results.
func Retrieve(feed string) (*RSSDocument, error) {
	if feed == "" {
		return nil, errors.New("No rss feed uri provided")
	}

	// Retrieve the rss feed document from the web.
	resp, err := http.Get(feed)
	if err != nil {
		return nil, err
	}

	// Close the response once we return from the function.
	defer closeResponse(resp)

	// Check the status code for a 200 so we know we have received a
	// proper response.
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP Response Error %d\n", resp.StatusCode)
	}

	// Decode the rss feed document into our struct type.
	// We don't need to check for errors, the caller can do this.
	var document RSSDocument
	err = xml.NewDecoder(resp.Body).Decode(&document)
	return &document, err
}

func closeResponse(resp *http.Response) {
	err := resp.Body.Close()
	if err != nil {
		fmt.Println("Could not close response: ", err)
	}
}
