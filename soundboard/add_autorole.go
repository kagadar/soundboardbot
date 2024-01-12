package soundboard

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/kagadar/go-pipeline"
	"k8s.io/klog/v2"
)

const (
	autoroleCommand        = "add-autorole"
	roleOption             = "role"
	templateRoleNameOption = "template_role_name"
)

func (b *bot) initAddAutorole() {
	b.commands[autoroleCommand] = command{
		&discordgo.ApplicationCommand{
			Description: "Adds an autorole for this server",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionRole,
					Name:        roleOption,
					Description: "The Role which will become an AutoRole",
					Required:    true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        templateRoleNameOption,
					Description: "The Name of the Role in the Soundboard Template which members in this AutoRole will be assigned to",
					Required:    true,
					Choices: pipeline.TransformSlice(b.roles.Elements(), func(r string) *discordgo.ApplicationCommandOptionChoice {
						return &discordgo.ApplicationCommandOptionChoice{Name: r, Value: r}
					}),
				},
			},
		}, b.addAutorole}
}

func (b *bot) addAutorole(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	if err := b.validateUser(user, autoroleCommand); err != nil {
		return err
	}

	roleID := options[roleOption].RoleValue(nil, "").ID
	roleTemplateName := options[templateRoleNameOption].StringValue()

	klog.Infof("add autorole for %q in %q to assign role %q requested by %q", roleID, interaction.GuildID, roleTemplateName, user)

	if err := b.db.InsertAutoRole(ctx, interaction.GuildID, roleID, roleTemplateName); err != nil {
		return err
	}

	if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("autorole for %q in %q will assign role %q", roleID, interaction.GuildID, roleTemplateName)),
	}); err != nil {
		return fmt.Errorf("%w: failed to notify %q of completed add autorole request", err, user)
	}
	klog.Infof("add autorole for %q in %q to assign role %q completed by %q", roleID, interaction.GuildID, roleTemplateName, user)
	return nil
}
