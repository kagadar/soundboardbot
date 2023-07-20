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

func deleteServer(s *discordgo.Session, event *discordgo.InteractionCreate) {
	user, err := interactionUser(event)
	if err != nil {
		glog.Error(err)
		return
	}
	username := username(user)
	if event.Type != discordgo.InteractionApplicationCommand {
		glog.Errorf("unexpected event type received by delete server handler: %s", event.Type.String())
		return
	}
	var guildID string
	for _, option := range event.ApplicationCommandData().Options {
		if option.Name != deleteServerIDOption {
			continue
		}
		if option.Type != discordgo.ApplicationCommandOptionString {
			glog.Errorf("unexpected option type for %q option: %s", deleteServerIDOption, option.Type.String())
			return
		}
		guildID = option.StringValue()
	}
	if guildID == "" {
		glog.Warning("received empty serverID, despite it being a required field")
		return
	}
	if err := s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
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
		if _, err := s.FollowupMessageCreate(event.Interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("I do not own server %q, so I cannot delete it.", guildID),
		}); err != nil {
			glog.Errorf("failed to notify %q of inability to complete delete request: %v", username, err)
		}
		return
	}
	glog.Infof("delete guild %q request received from %q", guildID, username)
	if _, err := s.GuildDelete(guildID); err != nil {
		if !errors.Is(err, discordgo.ErrJSONUnmarshal) {
			// DiscordGo incorrectly tries to unmarshal the response from the Guild Delete request.
			// This is doomed to fail, since the request returns `204 No Content`: https://discord.com/developers/docs/resources/guild#delete-guild
			glog.Errorf("failed to delete guild %q: %v", guildID, err)
			return
		}
	}
	if _, err := s.FollowupMessageCreate(event.Interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("%q has been deleted.", guildID),
	}); err != nil {
		glog.Errorf("failed to notify %q of completed delete request: %v", username, err)
		return
	}
	glog.Infof("guild %q has been deleted by %q", guildID, username)
}
