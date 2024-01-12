package soundboard

import (
	"context"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"k8s.io/klog/v2"
)

const (
	listServerCommand = "list-servers"
)

func (b *bot) initListServers() {
	b.commands[listServerCommand] = command{
		&discordgo.ApplicationCommand{
			Description: "Lists all servers that the bot owns",
		}, b.listServers}
}

func (b *bot) listServers(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	if err := b.validateUser(user, listServerCommand); err != nil {
		return err
	}

	klog.Infof("list guilds request received from %q", user)
	var guilds []string
	func() {
		b.creator.State.RLock()
		defer b.creator.State.RUnlock()
		for _, guild := range b.creator.State.Guilds {
			if guild.OwnerID == b.creator.State.User.ID {
				guilds = append(guilds, fmt.Sprintf("%q (%s)", guild.Name, guild.ID))
			}
		}
	}()

	var content string
	if len(guilds) == 0 {
		content = "I do not own any servers."
	} else {
		content = strings.Join(guilds, "\n")
	}
	if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr(content),
	}); err != nil {
		return fmt.Errorf("%w: failed to notify %q of completed list request", err, user)
	}
	klog.Infof("sent guild list to %q", user)
	return nil
}
