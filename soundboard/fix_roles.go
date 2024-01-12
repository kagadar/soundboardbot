package soundboard

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/kagadar/go-set"
	"k8s.io/klog/v2"
)

const (
	fixRolesCommand = "fix-roles"
)

func (b *bot) initFixRoles() {
	b.commands[fixRolesCommand] = command{
		&discordgo.ApplicationCommand{
			Description: "Fix AutoRoles for calling user",
		}, b.fixRolesCommand}
}

func (b *bot) findMainRoles(ctx context.Context, user *discordgo.User) (set.Set[discordgo.Snowflake], error) {
	guildIDs, err := b.db.ListGuilds(ctx)
	if err != nil {
		return nil, err
	}
	roles := set.New[discordgo.Snowflake]()
	for guildID := range guildIDs {
		member, err := b.manager.GuildMember(guildID, user.ID)
		if err != nil {
			// Just assume that an error here means that the user isn't part of this guild.
			klog.Warningf("%v: failed to look up membership of %q in %q", err, user, guildID)
			continue
		}
		for _, mr := range member.Roles {
			roles.Put(mr)
		}
	}
	return roles, nil
}

func (b *bot) fixRolesCommand(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	klog.Infof("fix roles requested by %q", user)
	mainRoles, err := b.findMainRoles(ctx, user)
	if err != nil {
		return err
	}
	requiredRoles, err := b.db.FindAllSoundboardRoles(ctx, mainRoles)
	if err != nil {
		return err
	}
	missingRoles := map[discordgo.Snowflake]set.Set[discordgo.Snowflake]{}
	for guildID, roleIDs := range requiredRoles {
		guild, err := b.manager.Guild(guildID)
		if err != nil {
			klog.Warningf("%v: failed to look up guild %q", err, guildID)
			continue
		}
		if guild.OwnerID == user.ID {
			continue
		}
		member, err := b.manager.GuildMember(guildID, user.ID)
		if err != nil {
			// Just assume that an error here means that the user isn't part of this guild.
			// This means we should invite them to the guild.
			klog.Warningf("%v: failed to look up membership of %q in %q", err, user, guildID)
			continue
		}
		for _, mr := range member.Roles {
			delete(roleIDs, mr)
		}
		missingRoles[guildID] = roleIDs
		for roleID := range roleIDs {
			if err := b.manager.GuildMemberRoleAdd(guildID, user.ID, roleID); err != nil {
				g, _ := b.manager.Guild(guildID)
				r, _ := b.manager.State.Role(guildID, roleID)
				return fmt.Errorf("%w: failed to grant %q role %q (%s) in %q (%s)", err, user, roleID, r.Name, guildID, g.Name)
			}
		}
	}

	if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("%q requires roles %+v", user, missingRoles)),
	}); err != nil {
		return fmt.Errorf("%w: failed to notify %q of completed fix roles request", err, user)
	}
	klog.Infof("fix roles for %q completed", user)
	return nil
}
