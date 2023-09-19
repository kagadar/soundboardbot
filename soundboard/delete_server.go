package soundboard

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

const (
	deleteServerIDOption = "server_id"
)

func (b *bot) initDeleteServer() {
	b.commands["delete-server"] = command{&discordgo.ApplicationCommand{Description: "Deletes a borked server", Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        deleteServerIDOption,
			Description: "The server to be deleted",
			Required:    true,
		},
	}}, b.deleteServer}
}

func (b *bot) deleteServer(interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption) error {
	guildID := options[deleteServerIDOption].StringValue()
	if err := b.soundboard.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		return fmt.Errorf("failed to respond to interaction request: %w", err)
	}
	var owned bool
	b.soundboard.State.RLock()
	for _, guild := range b.soundboard.State.Guilds {
		if guild.ID != guildID {
			continue
		}
		if guild.OwnerID != b.soundboard.State.User.ID {
			break
		}
		owned = true
	}
	b.soundboard.State.RUnlock()
	if !owned {
		if _, err := b.soundboard.FollowupMessageCreate(interaction, false, &discordgo.WebhookParams{
			Content: fmt.Sprintf("I do not own server %q, so I cannot delete it.", guildID),
		}); err != nil {
			return fmt.Errorf("failed to notify %q of inability to complete delete request: %w", user, err)
		}
		return nil
	}
	glog.Infof("delete guild %q request received from %q", guildID, user)
	if _, err := b.soundboard.GuildDelete(guildID); err != nil {
		if !errors.Is(err, discordgo.ErrJSONUnmarshal) {
			// DiscordGo incorrectly tries to unmarshal the response from the Guild Delete request.
			// This is doomed to fail, since the request returns `204 No Content`: https://discord.com/developers/docs/resources/guild#delete-guild
			return fmt.Errorf("failed to delete guild %q: %w", guildID, err)
		}
	}
	if _, err := b.soundboard.FollowupMessageCreate(interaction, false, &discordgo.WebhookParams{
		Content: fmt.Sprintf("%q has been deleted.", guildID),
	}); err != nil {
		return fmt.Errorf("failed to notify %q of completed delete request: %w", user, err)
	}
	glog.Infof("guild %q has been deleted by %q", guildID, user)
	return nil
}
