package goslackbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

type SlackBot struct {
	ID                string
	rtmToken          string
	wsURL             string
	users             map[string]SlackUser
	channels          map[string]SlackChannel
	groups            map[string]SlackChannel
	ims               map[string]SlackChannel // direct messages or im in slack lingo
	mpims             map[string]SlackChannel
	teams             map[string]SlackTeam
	ws                *websocket.Conn
	OutgoingMessages  chan SlackMessage
	IncomingMessages  map[string]chan SlackMessage
	IncomingFunctions map[string]func(SlackMessage)
	Conversations     map[string]SlackConversation
	ReactionCallbacks map[string]func(SlackMessage)
}

// type SlackReactionCallback func(channel, timestamp string)

type SlackAPIReactionAdd struct {
	Token     string `json:"token"`
	Name      string `json:"name"`
	Channel   string `json:"channel"`
	TimeStamp string `json:"timestamp"`
}

type SlackPostMessageResponse struct {
	Ok        bool   `json:"ok"`
	Channel   string `json:"channel"`
	TimeStamp string `json:"ts"`
}

type SlackConversation struct {
	Messages []SlackMessage
	Ongoing  bool
	State    string
	Started  time.Time
}

type ConversationMap map[string]SlackConversation

var counter uint64

func NewSlackBot(token string) (*SlackBot, error) {

	url := fmt.Sprintf("https://slack.com/api/rtm.start?mpim_aware=1&token=%s", token)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		log.Printf("Error Creating Bot: %s \n", err)
		return nil, err
	}

	var respObj SlackRTMResponse
	err = json.Unmarshal(body, &respObj)
	if err != nil {
		return nil, err
	}

	if !respObj.Ok {
		err = fmt.Errorf("Slack error: %s", respObj.Error)
		return nil, err
	}

	bot := SlackBot{}
	bot.SetURL(respObj.Url)
	bot.SetID((respObj.Self.Id))

	bot.channels = make(map[string]SlackChannel)
	for _, i := range respObj.Channels {
		bot.channels[i.Name] = i
		fmt.Printf("Channel: %s %s\n", i.ID, i.Name)
	}

	bot.teams = make(map[string]SlackTeam)
	for _, i := range respObj.Teams {
		bot.teams[i.Name] = i
		log.Printf("Team: %s %s\n", i.ID, i.Name)
	}
	bot.users = make(map[string]SlackUser)
	for _, u := range respObj.Users {
		bot.users[u.Name] = u
		// fmt.Printf("User: %s\t%s\n", u.ID, u.Name)
	}

	bot.mpims = make(map[string]SlackChannel)
	for _, mpim := range respObj.MPIMs {
		bot.channels[mpim.Name] = mpim
		// fmt.Printf("MPIM: %s\t%s\n", mpim.ID, mpim.Name)
	}

	bot.groups = make(map[string]SlackChannel)
	for _, group := range respObj.Groups {
		bot.channels[group.ID] = group
		// fmt.Printf("Group: %s\t%s\n", group.ID, group.Name)
	}

	bot.ims = make(map[string]SlackChannel)
	for _, im := range respObj.IMs {
		bot.ims[im.ID] = im
	}

	bot.OutgoingMessages = make(chan SlackMessage)
	bot.IncomingMessages = make(map[string]chan SlackMessage, 0)

	bot.rtmToken = token

	bot.ReactionCallbacks = make(map[string]func(SlackMessage))
	return &bot, nil
}

func (s *SlackBot) ReConnect() *websocket.Conn {
	for {
		url := fmt.Sprintf("https://slack.com/api/rtm.start?mpim_aware=1&token=%s", s.rtmToken)
		resp, err := http.Get(url)
		if err != nil {
			time.Sleep(time.Minute * 1)
			continue
		}
		if resp.StatusCode != 200 {
			err = fmt.Errorf("API request failed with code %d", resp.StatusCode)
			time.Sleep(time.Minute * 1)
			continue
		}
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			log.Printf("Error Reconnecting Bot: %s \n", err)
			time.Sleep(time.Minute * 1)
			continue
		}

		var respObj SlackRTMResponse
		err = json.Unmarshal(body, &respObj)
		if err != nil {
			log.Printf("Error Unmarshaling RTM Response : %s \n", err)
			time.Sleep(time.Minute * 1)
			continue
		}

		if !respObj.Ok {
			log.Printf("Slack error: %s", respObj.Error)
			time.Sleep(time.Minute * 1)
			continue
		}

		s.SetURL(respObj.Url)
		s.SetID((respObj.Self.Id))

		s.channels = make(map[string]SlackChannel)
		for _, i := range respObj.Channels {
			s.channels[i.Name] = i
			fmt.Printf("Channel: %s %s\n", i.ID, i.Name)
		}

		s.teams = make(map[string]SlackTeam)
		for _, i := range respObj.Teams {
			s.teams[i.Name] = i
			log.Printf("Team: %s %s\n", i.ID, i.Name)
		}
		s.users = make(map[string]SlackUser)
		for _, u := range respObj.Users {
			s.users[u.Name] = u
			// fmt.Printf("User: %s\t%s\n", u.ID, u.Name)
		}

		s.mpims = make(map[string]SlackChannel)
		for _, mpim := range respObj.MPIMs {
			s.channels[mpim.Name] = mpim
			// fmt.Printf("MPIM: %s\t%s\n", mpim.ID, mpim.Name)
		}

		s.groups = make(map[string]SlackChannel)
		for _, group := range respObj.Groups {
			s.channels[group.ID] = group
			// fmt.Printf("Group: %s\t%s\n", group.ID, group.Name)
		}

		s.ims = make(map[string]SlackChannel)
		for _, im := range respObj.IMs {
			s.ims[im.ID] = im
		}
		ws, err := websocket.Dial(s.wsURL, "", "https://api.slack.com/")
		if err == nil {
			s.ws = ws
			return ws
		}
		time.Sleep(time.Minute * 1)
	}
}
func (s *SlackBot) RemoveReactionCallback(channel, ts string) {
	key := strings.Join([]string{channel, ts}, "+")
	s.ReactionCallbacks[key] = nil
}

func (s *SlackBot) AddReactionCallback(channel, ts string, callback func(SlackMessage)) {

	key := strings.Join([]string{channel, ts}, "+")
	s.ReactionCallbacks[key] = callback
}

func (s *SlackBot) TriggerReactionCallback(m SlackMessage) error {

	key := strings.Join([]string{m.Channel, m.TimeStamp}, "+")
	if callback, ok := s.ReactionCallbacks[key]; ok {
		callback(m)
	}

	return nil
}

func (s *SlackBot) FetchReactionCallback(channel, timestamp string) func(m SlackMessage) {

	key := strings.Join([]string{channel, timestamp}, "+")

	if callback, ok := s.ReactionCallbacks[key]; ok {
		return callback
	}

	return func(m SlackMessage) {
		log.Println("DO NOTHING")
	}
}

func (s *SlackBot) GetUser(id string) SlackUser {

	return s.users[id]
}

func (s *SlackBot) GetChannel(id string) SlackChannel {

	if strings.HasPrefix(id, "G") {
		return s.groups[id]
	} else {
		if strings.HasPrefix(id, "D") {
			return s.ims[id]
		}
		return s.channels[id]
	}
}

func (s *SlackBot) GetChannelByName(name string) SlackChannel {
	if strings.HasPrefix(name, "G") {
		return s.groups[name]
	} else {
		if strings.HasPrefix(name, "D") {
			return s.ims[name]
		}
		return s.channels[name]
	}
}

func (s *SlackBot) RegisterIncomingChannel(name string, incoming chan SlackMessage) error {

	s.IncomingMessages[name] = incoming
	return nil
}

func (s *SlackBot) RegisterIncomingFunction(name string, runme func(SlackMessage)) {

	log.Printf("Registering Incoming Function %s", name)
	c := make(chan SlackMessage)
	s.RegisterIncomingChannel(name, c)

	go func() {
		for {
			m := <-c
			if m.Type != "" && m.Type != "error" && m.Type != "pong" {
				runme(m)
			}
		}
	}()
}

func getMessage(ws *websocket.Conn) (m SlackMessage, shouldReconnect bool, err error) {

	// err = websocket.JSON.Receive(ws, &m)
	var message string
	var retry time.Duration = 1
	// socket/network errors
	errors := []string{"EOF", "timed out", "network is down"}

	for {
		if err = websocket.Message.Receive(ws, &message); err != nil {
			log.Printf("Failed to receive websocket message %s \n", err)
			for _, e := range errors {
				if strings.HasSuffix(err.Error(), e) {
					shouldReconnect = true
					return // network/socket error, need to tear down the socket and start over.
				}
			}
			// transient error, exponential retry for n times and then just give up
			time.Sleep(time.Second * retry)
			retry = retry * 2
			if retry > 30 {
				// still having issues with the network, an error we are not accounting for yet, just give up and reconstruct connection
				shouldReconnect = true
				return
			}
		} else {
			break
		}
	}
	shouldReconnect = false // we may get further errors with the unmarshaling but that should not cause us to reconnect.

	err = json.Unmarshal([]byte(message), &m)
	// log.Printf("RAW %s\n", message)

	if m.Channel == "" && m.Item.Channel != "" {
		m.Channel = m.Item.Channel
		m.TimeStamp = m.Item.TimeStamp
	}

	return
}

func (s *SlackBot) PostMessage(channel, text string) (*SlackPostMessageResponse, error) {

	v := url.Values{}
	v.Set("token", s.rtmToken)
	v.Set("channel", channel)
	v.Set("text", text)

	req, err := http.NewRequest("GET", "https://slack.com/api/chat.postMessage?"+v.Encode(), nil)

	req.Header.Add("Content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	responseBody, err := ioutil.ReadAll(resp.Body)

	response := SlackPostMessageResponse{}
	err = json.Unmarshal(responseBody, &response)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (s *SlackBot) AddReaction(channel, timestamp, reaction string) error {

	v := url.Values{}
	v.Set("token", s.rtmToken)
	v.Set("name", reaction)
	v.Set("channel", channel)
	v.Set("timestamp", timestamp)

	req, err := http.NewRequest("GET", "https://slack.com/api/reactions.add?"+v.Encode(), nil)

	req.Header.Add("X-Conversation-ID", "0xf00f6")
	req.Header.Add("Content-type", "application/json")
	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	return nil
}

func (s *SlackBot) SendMessage(channel, message string) error {

	m := SlackMessage{
		Text:    message,
		Channel: channel,
		Type:    "message",
	}
	s.OutgoingMessages <- m
	return nil
}

func (s *SlackBot) Ping() error {
	m := SlackMessage{
		Text:    "ping",
		Type:    "ping",
		Channel: "",
	}
	// log.Printf("Queueing Ping message %s \n", time.Now())
	s.OutgoingMessages <- m
	return nil
}

func (s *SlackBot) SetID(id string) error {
	s.ID = id

	return nil
}

func (s *SlackBot) SetURL(url string) error {
	s.wsURL = url
	return nil
}

func (s *SlackBot) Connect() error {

	ws, err := websocket.Dial(s.wsURL, "", "https://api.slack.com/")
	if err != nil {
		return err
	}

	s.ws = ws

	go func() {
		for {
			m := <-s.OutgoingMessages
			channel := s.GetChannel(m.Channel)
			if m.Channel != "" {
				m.Id = atomic.AddUint64(&channel.LastMessageID, 1)
			}

			// if m.Type == "ping" {
			// 	log.Printf("pinging... %s \n", time.Now().String())
			// }
			var retry time.Duration = 1

			for {
				if err := websocket.JSON.Send(s.ws, m); err != nil {
					log.Printf("Error %v sending message %#v on websocket\n", err, m)
					if m.Type == "ping" {
						// throw away the ping if we are having network issues. wait on the socket to be reconstructed elsewhere.
						retry = 1
						break
					}
					time.Sleep(retry * time.Second)
					if retry < 60 {
						retry = retry * 2 // exponential retry
					}

				} else {
					retry = 1
					break
				}
			}

		}
	}()

	go func() {
		for {
			m, shouldReconnect, err := getMessage(ws)

			if err != nil && shouldReconnect {
				time.Sleep(time.Second * 1)
				log.Println("Reattempting WS reconstruction...")
				ws = s.ReConnect()

			} else {
				// send the incoming message to all registered listeners.
				for _, c := range s.IncomingMessages {
					c <- m
				}
			}
		}
	}()

	go func() {
		for {
			// log.Printf("Ping process starting...\n")
			s.Ping()
			time.Sleep(time.Second * 30)
		}
	}()
	return nil
}
