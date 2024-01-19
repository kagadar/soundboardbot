package soundboard

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/kagadar/go-pipeline/channels"
	"github.com/kagadar/go-syncmap"
	"k8s.io/klog/v2"
)

const (
	inviteCommand = "invite-me"
)

func (b *bot) initInvite() {
	b.commands[inviteCommand] = command{
		&discordgo.ApplicationCommand{
			Description: "Invite to all soundboards",
		}, b.inviteCommand}
	b.creatorHandlers = append(b.creatorHandlers, func(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
		b.grantAutoRoles(b.creator, event, &b.creatorInvites)
	})
	b.managerHandlers = append(b.managerHandlers, func(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
		b.grantAutoRoles(b.manager, event, &b.managerInvites)
	})
}

func (b *bot) grantAutoRoles(session *discordgo.Session, event *discordgo.GuildMemberAdd, pending *syncmap.Map[discordgo.Snowflake, pendingInvite]) {
	// Check if event has a related grant
	grant, ok := pending.Load(event.GuildID)
	if !ok {
		return
	}
	if grant.user != event.User.ID {
		return
	}
	// Claim the grant
	if _, ok := pending.LoadAndDelete(event.GuildID); !ok {
		// Another thread claimed this grant already
		return
	}
	klog.Infof("%q has joined %q, granting autoroles", event.User, event.GuildID)
	ctx := context.Background()
	var err error
	defer func() { grant.done <- err }()
	mainRoles, err := b.findMainRoles(ctx, event.User)
	if err != nil {
		return
	}
	neededRoles, err := b.db.FindSoundboardRoles(ctx, event.GuildID, mainRoles)
	if err != nil {
		return
	}
	for role := range neededRoles {
		if err = session.GuildMemberRoleAdd(event.GuildID, grant.user, role); err != nil {
			return
		}
	}
	klog.Infof("%q has been granted autoroles in %q", event.User, event.GuildID)
}

func createInvite(session *discordgo.Session, pending *syncmap.Map[discordgo.Snowflake, pendingInvite], guild *discordgo.Guild, userID discordgo.Snowflake) (*discordgo.Invite, chan error, error) {
	done := make(chan error)
	pending.Store(guild.ID, pendingInvite{user: userID, done: done})
	invite, err := session.ChannelInviteCreate(guild.SystemChannelID, discordgo.Invite{})
	if err != nil {
		return nil, nil, err
	}
	return invite, done, nil
}

func (b *bot) inviteCommand(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	klog.Infof("invite requested by %q", user)

	guildIDs, err := b.db.ListSoundboards(ctx)
	if err != nil {
		return err
	}
	for guildID := range guildIDs {
		if _, err := b.manager.GuildMember(guildID, user.ID); err != nil {
			actual := &discordgo.RESTError{}
			if !errors.As(err, &actual) {
				return fmt.Errorf("%w: failed to look up membership of %q in %q", err, user, guildID)
			}
			if actual.Message.Message == "Unknown Member" {
				guild, err := b.manager.Guild(guildID)
				if err != nil {
					return err
				}
				invite, done, err := createInvite(b.manager, &b.managerInvites, guild, user.ID)
				if err != nil {
					return err
				}
				if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
					Content: toPtr(fmt.Sprintf("https://discord.gg/%s", invite.Code)),
				}); err != nil {
					return fmt.Errorf("%w: failed to notify %q of invite request", err, user)
				}
				klog.Infof("waiting for %q to join %q", user, guild.ID)
				if err, _, _ := channels.Await(ctx, done); err != nil {
					return fmt.Errorf("%w: failed to wait for %q to join %q", err, user, guild.ID)
				}
			}
		}
	}
	if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr("done"),
	}); err != nil {
		return fmt.Errorf("%w: failed to notify %q of completed invite request", err, user)
	}
	klog.Infof("invite for %q completed", user)
	return nil
}
