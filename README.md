# goslackbot
A Slack Bot framework Written in Go

A simple RTM Slack Bot that allows you to either register a channel for new messages to be sent to you or to specify a callback function directly.

Example Application using this bot framework
```
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/andrewwatson/goslackbot"
)

var (
	bot            goslackbot.SlackBot
	rtmToken       string
	tomatoDuration int64
)

func main() {

	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: tomatobot slack-bot-token tomato-duration\n")
		os.Exit(1)
	}

	rtmToken = os.Args[1]
	tomatoArg := os.Args[2]

	tomatoDuration, err := strconv.Atoi(tomatoArg)
	if err != nil {
		fmt.Errorf("Invalid Number of Minutes for :tomato: %s", err.Error())
		os.Exit(1)
	}

	bot, err := goslackbot.NewSlackBot(rtmToken)

	if err != nil {
		log.Fatalf("CAN NOT BOT %s", err)
	}

	// fmt.Println(bot)

	bot.RegisterIncomingFunction("function", func(m goslackbot.SlackMessage) {
		if m.Type == "message" {

			user := bot.GetUser(m.User)
			channel := bot.GetChannel(m.Channel)

			log.Printf("Message from %s on %s", user.Name, channel.Name)

			if strings.HasPrefix(m.Text, ":tomato:") || strings.HasPrefix(m.Text, ":pomodoro:") {

				remindMe := strings.Replace(m.Text, ":tomato: ", "", 1)
				response := fmt.Sprintf("Ok <@%s>, got it! I'll remind you in %d minutes", m.User, tomatoDuration)

				bot.SendMessage(m.Channel, response)

				go func() {

					after := time.After(time.Minute * time.Duration(tomatoDuration))
					<-after

					response = fmt.Sprintf("Hey <@%s>, it's been %d minutes since you started '%s'", m.User, tomatoDuration, remindMe)
					bot.SendMessage(m.Channel, response)

				}()

			}

		}
	})

	err = bot.Connect()

	if err != nil {
		log.Fatalf("ERR: %s", err)
	}
}
```