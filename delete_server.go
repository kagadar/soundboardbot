package main

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

const (
	deleteServerIDOption = "server_id"
)

func init() {
	commands["delete-server"] = command{&discordgo.ApplicationCommand{Description: "Deletes a borked server", Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        deleteServerIDOption,
			Description: "The server to be deleted",
			Required:    true,
		},
	}}, deleteServer}
}

func deleteServer(s *discordgo.Session, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	guildID := options[deleteServerIDOption].StringValue()
	if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		glog.Errorf("failed to respond to interaction request: %v", err)
		return
	}
	var owned bool
	s.State.RLock()
	for _, guild := range s.State.Guilds {
		if guild.ID != guildID {
			continue
		}
		if guild.OwnerID != s.State.User.ID {
			break
		}
		owned = true
	}
	s.State.RUnlock()
	if !owned {
		if _, err := s.FollowupMessageCreate(interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("I do not own server %q, so I cannot delete it.", guildID),
		}); err != nil {
			glog.Errorf("failed to notify %q of inability to complete delete request: %v", user, err)
		}
		return
	}
	glog.Infof("delete guild %q request received from %q", guildID, user)
	if _, err := s.GuildDelete(guildID); err != nil {
		if !errors.Is(err, discordgo.ErrJSONUnmarshal) {
			// DiscordGo incorrectly tries to unmarshal the response from the Guild Delete request.
			// This is doomed to fail, since the request returns `204 No Content`: https://discord.com/developers/docs/resources/guild#delete-guild
			glog.Errorf("failed to delete guild %q: %v", guildID, err)
			return
		}
	}
	if _, err := s.FollowupMessageCreate(interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("%q has been deleted.", guildID),
	}); err != nil {
		glog.Errorf("failed to notify %q of completed delete request: %v", user, err)
		return
	}
	glog.Infof("guild %q has been deleted by %q", guildID, user)
}
