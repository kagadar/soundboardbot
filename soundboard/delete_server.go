package soundboard

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"k8s.io/klog/v2"
)

const (
	deleteServerCommand  = "delete-server"
	deleteServerIDOption = "server_id"
)

var (
	ErrServerNotOwned = errors.New("creator bot does not own server")
)

func (b *bot) initDeleteServer() {
	b.commands[deleteServerCommand] = command{&discordgo.ApplicationCommand{Description: "Deletes a borked server", Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        deleteServerIDOption,
			Description: "The server to be deleted",
			Required:    true,
		},
	}}, b.deleteServer}
}

func (b *bot) deleteServer(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	if err := b.validateUser(user, deleteServerCommand); err != nil {
		return err
	}

	guildID := discordgo.Snowflake(options[deleteServerIDOption].StringValue())
	klog.Infof("delete guild %q request received from %q", guildID, user)
	var owned bool
	func() {
		b.creator.State.RLock()
		defer b.creator.State.RUnlock()
		for _, guild := range b.creator.State.Guilds {
			if guild.ID != guildID {
				continue
			}
			if guild.OwnerID != b.creator.State.User.ID {
				break
			}
			owned = true
		}
	}()
	if !owned {
		return ErrServerNotOwned
	}
	if err := b.creator.GuildDelete(guildID); err != nil {
		if !errors.Is(err, discordgo.ErrJSONUnmarshal) {
			// DiscordGo incorrectly tries to unmarshal the response from the Guild Delete request.
			// This is doomed to fail, since the request returns `204 No Content`: https://discord.com/developers/docs/resources/guild#delete-guild
			return fmt.Errorf("failed to delete guild %q: %w", guildID, err)
		}
	}
	if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("%q has been deleted.", guildID)),
	}); err != nil {
		return fmt.Errorf("failed to notify %q of completed delete request: %w", user, err)
	}
	klog.Infof("guild %q has been deleted by %q", guildID, user)
	return nil
}
