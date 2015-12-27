/*
Package adultpoints tracks "adult points" for GroupMe users. If someone does something adult (think "responsible"),
they can request an "adult point" by using the "!adultme" trigger along with a reason, e.g. "!adultme for starting a
new job". Other users can then either "!award" them the point or "!reject" the point. The current leaderboard can be
output to GroupMe with the "!adults" trigger.

This bot assumes that you have a MongoDB server (mongod) running. It's built for MongoLab on Heroku, but any instance
of MongoDB can be used by setting the MONGOLAB_URI and MONGOLAB_DB environment variables to the URI and database name of
for your Mongo setup, respectively.
*/
package adultpoints

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sha1sum/golang_groupme_bot/bot"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

// Handler is an empty struct that's meant to be instantiated and passed to a bot.Command as the Handler field.
type Handler struct{}

// Database document schema setup
type (
	user struct {
		ID       bson.ObjectId `bson:"_id"`
		UserID   string        `bson:"userID"`
		Created  time.Time     `bson:"created"`
		Name     string        `bson:"name"`
		Points   int           `bson:"points"`
		Requests []request     `bson:"requests"`
	}

	request struct {
		Reference   string      `bson:"reference"`
		RequestedOn time.Time   `bson:"requestedOn"`
		Approved    bool        `bson:"approved"`
		Reason      string      `bson:"reason"`
		Approvals   []approval  `bson:"approvals"`
		Rejections  []rejection `bson:"rejections"`
	}

	approval struct {
		ApprovedByID string    `bson:"approvedByID"`
		ApprovedOn   time.Time `bson:"approvedOn"`
	}

	rejection struct {
		RejectedByID string    `bson:"rejectedByID"`
		RejectedOn   time.Time `bson:"rejectedOn"`
	}
)

// DB is the name of the MongoDB database
var DB string

// Handle is a satisfaction of the bot.Handler interface that's used to process IncomingMessages and output
// OutgoingMessages
func (handler Handler) Handle(term string, c chan []*bot.OutgoingMessage, message bot.IncomingMessage) {
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
	defer sess.Close()

	results := pointProcess(term, sess, message)
	if results != nil {
		c <- results
	}
}

// pointProcess determines which trigger is being requested. This bot is built to always accept commands as being at
// the beginning of the GroupMe message text
func pointProcess(term string, sess *mgo.Session, message bot.IncomingMessage) []*bot.OutgoingMessage {
	words := strings.Split(message.Text[1:], " ")
	switch strings.ToLower(words[0]) {
	case "adultme":
		return requestPoint(words[1:], sess, message)
	case "award":
		return awardPoint(words[1:2], sess, message)
	case "reject":
		return rejectPoint(words[1:2], sess, message)
	case "adults":
		return listAdults(sess)
	default:
		return nil
	}
}

// requestPoint handles users making the request for a point.
func requestPoint(words []string, sess *mgo.Session, message bot.IncomingMessage) []*bot.OutgoingMessage {
	args := strings.Join(words, " ")
	col := sess.DB(DB).C("groupmeUsersV3")
	var cu user
	fmt.Println(message.UserID)
	err := col.Find(bson.M{"userID": message.UserID}).One(&cu)
	if err != nil {
		col.Insert(user{ID: bson.NewObjectId(), UserID: message.UserID, Name: message.Name, Points: 0, Created: time.Now()})
	}
	_ = col.Find(bson.M{"userID": message.UserID}).One(&cu)
	requests := cu.Requests
	reference := determineReference(cu, sess)
	cu.Requests = append(requests, request{
		Reference:   reference,
		RequestedOn: time.Now(),
		Approved:    false,
		Reason:      args,
	})
	col.Update(bson.M{"_id": cu.ID}, cu)
	t := message.Name + " has requested an adult point \"" + args + "\"."
	t += " To approve the point, just type \"!award " + reference + "\", or to reject it, use \"!reject " + reference + "\"."
	return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: t}}
}

// determineReference checks the award/reject trigger's corresponding reference number to determine which requested
// point is being awarded or rejected
func determineReference(cu user, sess *mgo.Session) string {
	var results []user
	_ = sess.DB(DB).C("groupmeUsersV3").Find(nil).Sort("created").All(&results)
	ui := 0
	for i, v := range results {
		if v.UserID == cu.UserID {
			ui = i
			break
		}
	}
	return strconv.Itoa(ui+1) + strconv.Itoa(len(cu.Requests)+1)
}

// awardPoint increases the number of approved point requests (as long as it's not a duplicate or an attempt to award
// a point by the requester or a bot)
func awardPoint(words []string, sess *mgo.Session, message bot.IncomingMessage) []*bot.OutgoingMessage {
	args := strings.Join(words, " ")
	col := sess.DB(DB).C("groupmeUsersV3")
	var cu user
	err := col.Find(bson.M{"requests": bson.M{"$elemMatch": bson.M{"reference": args}}}).One(&cu)
	if err != nil {
		return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "Couldn't find a request with reference \"" + args + "\"."}}
	}
	var ri int
	requests := cu.Requests
	for i, v := range requests {
		if v.Reference == args {
			ri = i
			break
		}
	}
	if cu.UserID == message.UserID || message.SenderType == "bot" {
		t := "Stop trying to be slick! You can't approve your own requests!"
		t += " Just for that, I'm revoking the request!"
		cu.Requests = append(requests[:ri], requests[ri+1:]...)
		col.Update(bson.M{"_id": cu.ID}, cu)
		return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: t}}
	}
	previous := [2]int{len(requests[ri].Approvals), len(requests[ri].Rejections)}
	for _, v := range requests[ri].Approvals {
		if v.ApprovedByID == message.UserID {
			return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "You've already approved that request (dumbass)."}}
		}
	}
	for i, v := range requests[ri].Rejections {
		if v.RejectedByID == message.UserID {
			requests[ri].Rejections = append(requests[ri].Rejections[:i], requests[ri].Rejections[i+1:]...)
			addApproval(col, message.UserID, &cu, ri)
			return []*bot.OutgoingMessage{
				&bot.OutgoingMessage{Text: "Your previous rejection has been switched to an approval (make up your damn mind)."},
				announcePointChange(true, col, &cu, &requests[ri], previous, message),
			}
		}
	}
	addApproval(col, message.UserID, &cu, ri)
	cu.Requests = requests
	return []*bot.OutgoingMessage{announcePointChange(true, col, &cu, &requests[ri], previous, message)}
}

// addApproval actually adds the point award to the DB document
func addApproval(col *mgo.Collection, approvedByID string, cu *user, ri int) {
	cu.Requests[ri].Approvals = append(cu.Requests[ri].Approvals, approval{ApprovedByID: approvedByID, ApprovedOn: time.Now()})
	col.Update(bson.M{"_id": cu.ID}, cu)
}

// rejectPoint handles rejecting a request for a point (as long as it's not a duplicate)
func rejectPoint(words []string, sess *mgo.Session, message bot.IncomingMessage) []*bot.OutgoingMessage {
	args := strings.Join(words, " ")
	col := sess.DB(DB).C("groupmeUsersV3")
	var cu user
	err := col.Find(bson.M{"requests": bson.M{"$elemMatch": bson.M{"reference": args}}}).One(&cu)
	if err != nil {
		return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "Couldn't find a request with reference \"" + args + "\"."}}
	}
	var ri int
	requests := cu.Requests
	for i, v := range requests {
		if v.Reference == args {
			ri = i
			break
		}
	}
	previous := [2]int{len(requests[ri].Approvals), len(requests[ri].Rejections)}
	if cu.UserID == message.UserID || message.SenderType == "bot" {
		t := "Uhhh, okay. If you really want to reject your own request, whatever. Wish granted."
		addRejection(col, message.UserID, &cu, ri)
		return []*bot.OutgoingMessage{
			&bot.OutgoingMessage{Text: t},
			announcePointChange(false, col, &cu, &requests[ri], previous, message),
		}
	}
	for _, v := range requests[ri].Rejections {
		if v.RejectedByID == message.UserID {
			return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: "You've already rejected that request (dumbass)."}}
		}
	}
	for i, v := range requests[ri].Approvals {
		if v.ApprovedByID == message.UserID {
			requests[ri].Approvals = append(requests[ri].Approvals[:i], requests[ri].Approvals[i+1:]...)
			addRejection(col, message.UserID, &cu, ri)
			return []*bot.OutgoingMessage{
				&bot.OutgoingMessage{Text: "Your previous approval has been switched to a rejection (make up your damn mind)."},
				announcePointChange(false, col, &cu, &requests[ri], previous, message),
			}
		}
	}
	addRejection(col, message.UserID, &cu, ri)
	cu.Requests = requests
	return []*bot.OutgoingMessage{announcePointChange(false, col, &cu, &requests[ri], previous, message)}
}

// addRejection adds the point rejection to the DB document
func addRejection(col *mgo.Collection, rejectedByID string, cu *user, ri int) {
	cu.Requests[ri].Rejections = append(cu.Requests[ri].Rejections, rejection{RejectedByID: rejectedByID, RejectedOn: time.Now()})
	col.Update(bson.M{"_id": cu.ID}, cu)
}

// announcePointChange sends a message to the group about the current state of the awards/rejects for the point request
// depending on the balance of awards/rejections
func announcePointChange(approving bool, col *mgo.Collection, cu *user, req *request, previous [2]int, message bot.IncomingMessage) *bot.OutgoingMessage {
	pa := previous[0]
	pr := previous[1]
	switch {
	case pa == 0 && pr == 0 && len(req.Approvals) == 1:
		cu.Points++
		col.Update(bson.M{"_id": cu.ID}, cu)
		return &bot.OutgoingMessage{Text: cu.Name + ", you just got your first point \"" + req.Reason + "\" (for now)!"}
	case pa == 0 && pr == 0 && len(req.Rejections) == 1:
		return &bot.OutgoingMessage{Text: "DENIED, " + cu.Name + " :( -- " + message.Name + " doesn't seem to believe you deserve your point \"" + req.Reason + "\"."}
	case pa <= pr && len(req.Approvals) > len(req.Rejections):
		cu.Points++
		col.Update(bson.M{"_id": cu.ID}, cu)
		return &bot.OutgoingMessage{Text: message.Name + " believes in you, " + cu.Name + "! You just got your point \"" + req.Reason + "\"!"}
	case pa > pr && len(req.Rejections) >= len(req.Approvals):
		cu.Points--
		col.Update(bson.M{"_id": cu.ID}, cu)
		return &bot.OutgoingMessage{Text: message.Name + " thinks you should try harder, " + cu.Name + "! Your point just got revoked \"" + req.Reason + "\". :("}
	case pr > pa && len(req.Rejections) == len(req.Approvals):
		return &bot.OutgoingMessage{Text: "So close to gettin' that point, " + cu.Name + "! You just need one more approval \"" + req.Reason + "\"."}
	case len(req.Approvals) > len(req.Rejections):
		return &bot.OutgoingMessage{Text: cu.Name + " is stackin' up approvals \"" + req.Reason + "\"!"}
	case len(req.Rejections) > len(req.Approvals) && len(req.Approvals) > pa:
		return &bot.OutgoingMessage{Text: "Still have some work to do to get that point, " + cu.Name + ", \"" + req.Reason + "\"."}
	case len(req.Rejections) > len(req.Approvals):
		return &bot.OutgoingMessage{Text: "Maybe you should rethink the meaning of \"adult\", " + cu.Name + ". More people disapprove of your point than agree \"" + req.Reason + "\"."}
	}
	return &bot.OutgoingMessage{Text: "I have no idea what's going on here."}
}

// listAdults outputs the current point leaderboard to the GroupMe group.
func listAdults(sess *mgo.Session) []*bot.OutgoingMessage {
	var results []user
	_ = sess.DB(DB).C("groupmeUsersV3").Find(nil).Sort("-points").All(&results)
	board := ""
	total := 0
	for _, v := range results {
		board += v.Name + ": " + strconv.Itoa(v.Points) + "\n"
		total += v.Points
	}
	board += "\nTOTAL: " + strconv.Itoa(total)
	return []*bot.OutgoingMessage{&bot.OutgoingMessage{Text: board}}
}
