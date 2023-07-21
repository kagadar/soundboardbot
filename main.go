package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

type command struct {
	command *discordgo.ApplicationCommand
	handler func(*discordgo.Session, *discordgo.Interaction, *discordgo.User, map[string]*discordgo.ApplicationCommandInteractionDataOption)
}

type token struct{}

var (
	accessToken = flag.String("discord_access_token", "", "Token used by the bot to access Discord")
	appID       = flag.String("app_id", "1131203534117937182", "The Application ID that commands should be registered with")
	superAdmins = flag.String("super_admins", "kagadar", "Comma-separated list of bot super admins")

	commands      = map[string]command{}
	extraHandlers = []interface{}{}
)

func init() { flag.Parse() }

func main() {
	s, err := discordgo.New(fmt.Sprintf("Bot %s", *accessToken))
	if err != nil {
		glog.Fatalf("failed to create session: %v", err)
	}
	s.Identify.Intents |= discordgo.IntentGuildMembers

	if err := s.Open(); err != nil {
		glog.Fatalf("failed to connect to discord: %v", err)
	}
	defer s.Close()

	for name, command := range commands {
		command.command.Name = name
		if _, err := s.ApplicationCommandCreate(*appID, "", command.command); err != nil {
			glog.Fatalf("failed to create application command %q: %v", name, err)
		}
	}
	s.AddHandler(func(s *discordgo.Session, event *discordgo.InteractionCreate) {
		if event.Type != discordgo.InteractionApplicationCommand {
			return
		}
		command, ok := commands[event.ApplicationCommandData().Name]
		if !ok {
			glog.Warningf("received command for unexpected interaction type: %q\n%+v", event.ApplicationCommandData().Name, event)
			return
		}
		var user *discordgo.User
		if event.User != nil {
			user = event.User
		} else {
			if event.Member == nil {
				glog.Errorf("interaction request recevied without any identified user: %+v", event)
				return
			}
			user = event.Member.User
		}
		options := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
		for _, option := range event.ApplicationCommandData().Options {
			options[option.Name] = option
		}
		command.handler(s, event.Interaction, user, options)
	})
	for _, handler := range extraHandlers {
		s.AddHandler(handler)
	}

	s.State.RLock()
	glog.Infof("Server started as %q", s.State.User.ID)
	s.State.RUnlock()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
}
