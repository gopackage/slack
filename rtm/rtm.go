// Package rtm implements the Slock real-time messaging API.
package rtm

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// DefaultServeMux is the default ServeMux and used by Serve.
// We are following the patterns in net/http but right now we don't build out
// all the underlying infrastructure - for now the default is the only mux.
var DefaultServeMux = NewServeMux()

// NewServeMux creates a new ServeMux.
func NewServeMux() *ServeMux {
	return &ServeMux{m: make(map[string]eventHandler)}
}

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as event handlers.  If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler object that calls f.
type HandlerFunc func(ResponseWriter, interface{})

// HandleEvent calls f(w, r).
func (f HandlerFunc) HandleEvent(w ResponseWriter, e interface{}) {
	f(w, e)
}

// eventHandler wraps registered event handlers with some extra meta-data
// to make event routing easier.
type eventHandler struct {
	handler Handler
	pattern string
}

// ServeMux is an RTM event multixplexer. It matches incoming RTM events
// by type and calls the handler that most closely matches the pattern.
// Pattern matching resolves to the "best" match (most precise).
// Handlers that register identical patterns will be dispatched to by random.
type ServeMux struct {
	mu sync.RWMutex
	m  map[string]eventHandler
}

// Handle adds a Handler that will be dispatched when any event that matches
// the provided pattern is received.
func (mux *ServeMux) Handle(pattern string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	e := eventHandler{handler: handler, pattern: pattern}
	mux.m[pattern] = e
}

// HandleFunc adds a handler that will be dispatched when an event that
// matches the provided pattern is received. The redundant functionality
// matches net/http and makes up for the difference in Go between anonmyous
// functions and interfaces.
func (mux *ServeMux) HandleFunc(pattern string, handler func(resp ResponseWriter, event interface{})) {
	mux.Handle(pattern, HandlerFunc(handler))
}

// Handler determines the correct handler to match a provided event. The
// handler return can be nil indicating no handlers are registered for
// the provided pattern. If the handler is non-nil the matching pattern
// is also returned (for debugging/testing).
func (mux *ServeMux) Handler(event interface{}) (h Handler, pattern string) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	// Currently we only support exact pattern matches. Would be nice to
	// at least add wild cards at some point or regular expressions.
	eType := event.(map[string]interface{})["type"].(string)
	e, ok := mux.m[eType]
	if ok {
		return e.handler, e.pattern
	}
	return nil, ""
}

// HandleEvent handles any incoming event from an RTM stream. Responses
// may be written to the ResponseWritter (but is not required).
func (mux *ServeMux) HandleEvent(resp ResponseWriter, event interface{}) {
	// Can do some pre-processing, logging, stats, etc here...
	h, _ := mux.Handler(event)
	if h != nil {
		h.HandleEvent(resp, event)
	}
}

// ResponseWriter interface provides the methods for Handlers to write
// to an active rtm connection.
type ResponseWriter interface {
	// Write sends the data to the connection as part of an RTM reply.
	// The event object must be JSON serializable.
	// Returns the number of bytes written or an error.
	Write(event map[string]interface{}) (int, error)
	// WriteMsg sends a simple RTM message. This is a simple convenience
	// for sending message objects to the RTM server.
	WriteMsg(channel, text string) (int, error)
}

// Handler interface should be implemented by any object that wants to
// receive events for a particular event type.
//
// HandleEvent may write zero or more responses to ResponseWriter and then
// return. Returning signals that the request is finished and that the event
// server can move on to the next request on the connection.
//
// If HandleEvent panics, the server (the caller of HandleEvent)
// assumes that the effect of the panic was isolated to the active request.
// It recovers the panic, logs a stack trace to the server error log, and
// continues received events.
type Handler interface {
	HandleEvent(resp ResponseWriter, event interface{})
}

// Client is a Slack Real-Time Messaging (RTM) client.
//
// Clients contain state information so they should be created instead of
// reused.
type Client struct {
	ws     *websocket.Conn
	sendID int64
}

// DialAndListen opens a connection to the Slack RTM server and begins
// handling incoming events using the provided handler. The method blocks
// so should be called in a goroutine if other processing needs to be done.
// If other threads wish to send messages without being triggered by incoming
// events, a handler should be registered for the "hello" event. When the
// hello event is received the RTM connection has been received and the
// ResponseWriter can be saved and used to send messages.
func (c *Client) DialAndListen(token string, handler Handler) (err error) {
	// Hit the rtm.start endpoint and get the websocket
	log.Println("rtm.start")
	resp, err := http.Get("https://slack.com/api/rtm.start?token=" + token)
	if err != nil {
		return err
	}
	log.Println("rtm.started")
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	log.Println("rtm.start body", len(body))

	var r StartResponse
	err = json.Unmarshal(body, &r)
	if err != nil {
		return err
	}
	log.Println("rtm.start body parsed", r.Ok, r.Error, r.URL)

	if !r.Ok {
		return fmt.Errorf("RTM API was not OK to start stream: %s", r.Error)
	}

	origin := os.Getenv("BITBOT_ORIGIN")
	log.Println("rtm.start origin", origin)
	c.ws, err = websocket.Dial(r.URL, "", origin)
	if err != nil {
		log.Println("rtm.start encountered websocket.Dial", err)
		return err
	}
	log.Println("rtm.start ws dialed")

	defer c.ws.Close()

	// Listen to the connection sending events to the event handler.
	msg := make([]byte, 4096)
	watchdog := time.AfterFunc(25*time.Second, func() {
		c.Write(map[string]interface{}{"type": "ping"})
	})

	log.Println("rtm.start ready to read event")
	for {
		var read int
		for read, err = c.ws.Read(msg); read == 4096 || err != nil; read, err = c.ws.Read(msg) {
			// Buffer not big enough - we read until drained
			if read == 0 {
				// This can loop infinitely fast with read == 0 so we will
				// sleep so we don't use up all the available CPU.
				log.Println("rtm.start ######### ws timeout")
				time.Sleep(1 * time.Second)
			} else {
				log.Println("rtm.start reading event", read)
			}
		}
		watchdog.Reset(25 * time.Second)
		var event interface{}
		err = json.Unmarshal(msg[0:read], &event)
		if err != nil {
			// packet no good, we ignore it for now
			log.Println("rtm.start ###### error parsing event", string(msg[0:read]), err)
		} else {
			log.Println("rtm.start handling event", string(msg[0:read]))
			handler.HandleEvent(c, event)
		}
	}
}

// Write sends the provided msg to the RTM server. All msgs must contain
// a "type" field. The "id" field will be automatically configured by the client.
func (c *Client) Write(msg map[string]interface{}) (int, error) {
	msg["id"] = c.sendID
	c.sendID++
	log.Printf("rtm.start write %v", msg)
	data, err := json.Marshal(msg)
	if err != nil {
		return -1, err
	}
	return c.ws.Write(data)
}

// WriteMsg is a simple convenience for sending RTM simple text messages.
// The "id" field will be automatically configured by the client.
func (c *Client) WriteMsg(channel, text string) (int, error) {
	return c.Write(map[string]interface{}{"type": "message", "channel": channel, "text": text})
}

// Handle adds a handler for an event on the DefaultServeMux.
// See ServeMux documentation for usage.
func Handle(pattern string, handler Handler) {
	DefaultServeMux.Handle(pattern, handler)
}

// HandleFunc adds a handler functino for an event on the DefaultServeMux.
// See ServeMux documentation for usage.
func HandleFunc(pattern string, handler func(resp ResponseWriter, event interface{})) {
	DefaultServeMux.HandleFunc(pattern, handler)
}

// DialAndListen opens a connection to the Slack RTM server and begins
// handling incoming events using the DefaultServeMux. The method blocks
// so should be called in a goroutine if other processing needs to be done.
// If other threads wish to send messages without being triggered by incoming
// events, a handler should be registered for the "hello" event. When the
// hello event is received the RTM connection has been received and the
// ResponseWriter can be saved and used to send messages.
func DialAndListen(token string) (err error) {
	client := Client{}
	return client.DialAndListen(token, DefaultServeMux)
}

// StartResponse is received from the Slack rtm.start API.
type StartResponse struct {
	// Ok is true if the RTM stream can begin
	Ok bool `json:"ok"`
	// Error contains an error message if Ok is false
	Error string `json:"error,omitempty"`
	// URL is the web socket to connect to (must be used within 30 sec)
	// e.g. "wss:\/\/ms9.slack-msgs.com\/websocket\/7I5yBpcvk"
	URL string `json:"url"`

	// TODO these should be a "database"
	//Self Self `json:"self"`
	//Team Team `json:"team"`
	//Users []string `json:"users"`
	//Channels []string `json:"channels"`
	//Groups   []string `json:"groups"`
	//IMs      []string `json:"ims"`
	//Bots []string `json:"bots"`
}

// Self describes the user's account
type Self struct {
	// ID uuid for the user e.g. "U023BECGF",
	ID string `json:"id"`
	// Name of the user e.g. "bobby"
	Name string `json:"name"`
	// Preferences for the user
	Preferences Preferences `json:"prefs"`
	// Timestamp the user's account was created e.g. 1402463766
	Created int64 `json:"created"`
	// ManualPresence indicates the presence mode for the user (active, manual)
	ManualPresence string `json:"manual_presence"`
}

// Preferences contains information about the preferences set for the parent object
type Preferences map[string]interface{}

// Team contains information on the teams the user belongs to.
type Team struct {
	// ID is the uuid for the team e.g. T024BE7LD
	ID string `json:"id"`
	// Name is the name of the slack team
	Name string `json:"name"`
	// EmailDomain is the slack default email domain for team members (can be empty)
	EmailDomain string `json:"email_domain"`
	// Domain is the slack domain for the current team
	Domain string `json:"domain"`
	// MsgEditWindowMins is the number of minutes for the last message to be editable or -1
	MsgEditWindowMins int `json:"msg_edit_window_mins"`
	// OverStorageLimit is true if the account is over the storage limit
	OverStorageLimit bool `json:"over_storage_limit"`
	// Preferences for the user
	Preferences Preferences `json:"prefs"`
	// Plan contains the current billing plan for the team (std, pro, etc)
	Plan string `json:"plan"`
}
