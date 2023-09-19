package soundboard

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

func (b *bot) initListServers() {
	b.commands["list-servers"] = command{&discordgo.ApplicationCommand{Description: "Lists all servers that the bot owns"}, b.listServers}
}

func (b *bot) listServers(interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption) error {
	glog.Infof("list guilds request received from %q", user)

	b.soundboard.State.RLock()
	var guilds []string
	for _, guild := range b.soundboard.State.Guilds {
		if guild.OwnerID == b.soundboard.State.User.ID {
			guilds = append(guilds, fmt.Sprintf("%q (%s)", guild.Name, guild.ID))
		}
	}
	b.soundboard.State.RUnlock()

	var content string
	if len(guilds) == 0 {
		content = "I do not own any servers."
	} else {
		content = strings.Join(guilds, "\n")
	}
	if err := b.soundboard.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		return fmt.Errorf("failed to respond to interaction request: %w", err)
	}
	glog.Infof("sent guild list to %q", user)
	return nil
}
