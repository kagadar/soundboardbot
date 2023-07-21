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

func listServers(s *discordgo.Session, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	glog.Infof("list guilds request received from %q", user)

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
	if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		glog.Errorf("failed to respond to interaction request: %v", err)
		return
	}
	glog.Infof("sent guild list to %q", user)
}
