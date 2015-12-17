package goslackbot

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync/atomic"

	"golang.org/x/net/websocket"
)

type SlackBot struct {
	ID                string
	rtmToken          string
	wsURL             string
	users             map[string]SlackUser
	channels          map[string]SlackChannel
	ws                *websocket.Conn
	OutgoingMessages  chan SlackMessage
	IncomingMessages  map[string]chan SlackMessage
	IncomingFunctions map[string]func(SlackMessage)
}

var counter uint64

func NewSlackBot(token string) (*SlackBot, error) {

	url := fmt.Sprintf("https://slack.com/api/rtm.start?token=%s", token)
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
		bot.channels[i.ID] = i
		fmt.Printf("Channel Listed: %s %s\n", i.ID, i.Name)
	}

	bot.users = make(map[string]SlackUser)
	for _, u := range respObj.Users {
		bot.users[u.ID] = u
		fmt.Printf("User: %s %s\n", u.ID, u.Name)
	}

	bot.OutgoingMessages = make(chan SlackMessage)
	bot.IncomingMessages = make(map[string]chan SlackMessage, 0)

	return &bot, nil
}

func (s *SlackBot) GetUser(id string) SlackUser {

	return s.users[id]
}

func (s *SlackBot) GetChannel(id string) SlackChannel {

	return s.channels[id]
}

func (s *SlackBot) RegisterIncomingChannel(name string, incoming chan SlackMessage) error {

	log.Printf("Registering Channel %s", name)
	s.IncomingMessages[name] = incoming

	return nil
}

func (s *SlackBot) RegisterIncomingFunction(name string, runme func(SlackMessage)) {

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
	err = websocket.JSON.Receive(ws, &m)
	return
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

	for {
		m, err := getMessage(ws)

		if err != nil {
			fmt.Errorf("Incoming Error: %s", err)
		}

		// log.Printf("INCOMING MESSAGE: %s", m.Text)
		for _, c := range s.IncomingMessages {
			c <- m
		}
	}

	return nil
}
