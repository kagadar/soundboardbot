package main

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

func init() {
	commands["list-servers"] = command{&discordgo.ApplicationCommand{Description: "Lists all servers that the bot owns"}, listServers}
}

func listServers(s *discordgo.Session, event *discordgo.InteractionCreate) {
	user, err := interactionUser(event)
	if err != nil {
		glog.Error(err)
		return
	}
	username := username(user)
	glog.Infof("list guilds request received from %q", username)
	s.State.RLock()
	var guilds []string
	for _, guild := range s.State.Guilds {
		if guild.OwnerID == s.State.User.ID {
			guilds = append(guilds, fmt.Sprintf("%q (%s)", guild.Name, guild.ID))
		}
	}
	s.State.RUnlock()
	var content string
	if len(guilds) == 0 {
		content = "I do not own any servers."
	} else {
		content = strings.Join(guilds, "\n")
	}
	if err := s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		glog.Errorf("failed to respond to interaction request: %v", err)
		return
	}
	glog.Infof("sent guild list to %q", username)
}
