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

	bot.OutgoingMessages = make(chan SlackMessage)
	bot.IncomingMessages = make(map[string]chan SlackMessage, 0)

	bot.rtmToken = token

	bot.ReactionCallbacks = make(map[string]func(SlackMessage))
	return &bot, nil
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
		return s.channels[id]
	}
}

func (s *SlackBot) GetChannelByName(name string) SlackChannel {
	if strings.HasPrefix(name, "G") {
		return s.groups[name]
	} else {
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
			runme(m)
		}
	}()
}

func getMessage(ws *websocket.Conn) (m SlackMessage, err error) {

	// err = websocket.JSON.Receive(ws, &m)
	var message string
	websocket.Message.Receive(ws, &message)

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
			m.Id = atomic.AddUint64(&counter, 1)
			websocket.JSON.Send(s.ws, m)

		}
	}()

	go func() {
		for {
			m, err := getMessage(ws)

			if err != nil {
				fmt.Errorf("Incoming Error: %s", err)
			}

			// log.Printf("INCOMING MESSAGE: %s", m.Type)
			for _, c := range s.IncomingMessages {
				c <- m
			}
		}
	}()

	return nil
}
