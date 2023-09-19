package soundboard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

const (
	createSoundboardSuffixOption = "server_suffix"
)

func (b *bot) initCreateSoundboard() {
	b.commands["create-soundboard"] = command{&discordgo.ApplicationCommand{
		Description: "Creates a soundboard and assigns ownership to the calling user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        createSoundboardSuffixOption,
				Description: "The suffix to add to the created server",
			},
		},
	}, b.createSoundboard}
	b.extraHandlers = append(b.extraHandlers, b.grantOwner)
}

func (b *bot) createSoundboard(interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption) error {
	glog.Infof("create guild request received from %q", user)

	if err := b.soundboard.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		return fmt.Errorf("failed to respond to interaction request: %w", err)
	}
	msg, err := b.soundboard.FollowupMessageCreate(interaction, true, &discordgo.WebhookParams{
		Content: "Working...",
	})
	if err != nil {
		return fmt.Errorf("failed to send follow-up message: %w", err)
	}
	var suffix string
	if options[createSoundboardSuffixOption] != nil {
		suffix = " " + strings.Join(strings.Split(strconv.Itoa(int(options[createSoundboardSuffixOption].IntValue())), ""), " ")
	}
	guild, err := b.soundboard.GuildCreateWithTemplate(b.template, fmt.Sprintf("soundboardhost%s", suffix), "")
	if err != nil {
		mainErr := fmt.Errorf("failed to create guild: %w", err)
		_, err := b.soundboard.FollowupMessageEdit(interaction, msg.ID, &discordgo.WebhookEdit{
			Content: toPtr("Failed to create Server"),
		})
		if err != nil {
			mainErr = fmt.Errorf("failed to notify user of error: %w", mainErr)
		}
		return mainErr
	}
	glog.Infof("created guild: %q\ndefault invite channel is %q", guild.ID, guild.SystemChannelID)
	glog.Infof("finding admin role for %q...", guild.ID)
	var admin *discordgo.Role
	for _, role := range guild.Roles {
		if role.Name == b.adminRole {
			admin = role
			glog.Infof("found admin role in %q: %q", guild.ID, role.ID)
		}
	}
	if admin == nil {
		return fmt.Errorf("no admin role found in %q", guild.ID)
	}
	done := make(chan token)
	b.pendingGrants.Store(guild.ID, &usergrant{user: user.ID, role: admin.ID, done: done})
	invite, err := b.soundboard.ChannelInviteCreate(guild.SystemChannelID, discordgo.Invite{})
	if err != nil {
		return fmt.Errorf("failed to create guild invite: %w", err)
	}
	if err := b.db.SaveInvite(guild.ID, invite.Code, invite.ExpiresAt); err != nil {
		return fmt.Errorf("failed to save guild invite: %w", err)
	}
	_, err = b.soundboard.FollowupMessageEdit(interaction, msg.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("https://discord.gg/%s", invite.Code)),
	})
	if err != nil {
		return fmt.Errorf("failed to update follow-up message: %w", err)
	}
	return nil
}

func (b *bot) grantOwner(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
	grant, ok := b.pendingGrants.Load(event.GuildID)
	if !ok {
		return
	}
	if grant.user != event.User.ID {
		return
	}

	glog.Infof("%q has joined %q, granting admin", event.User, event.GuildID)
	defer func() { grant.done <- token{} }()

	if err := b.soundboard.GuildMemberRoleAdd(event.GuildID, grant.user, grant.role); err != nil {
		glog.Errorf("failed to grant admin to user: %v", err)
	}
	if _, err := b.soundboard.GuildEdit(event.GuildID, &discordgo.GuildParams{OwnerID: event.User.ID}); err != nil {
		glog.Errorf("failed to change guild owner: %v", err)
	}
	if err := b.soundboard.GuildLeave(event.GuildID); err != nil {
		glog.Errorf("failed to leave guild: %w", err)
	}
	glog.Infof("%q has been granted ownership of %q", event.User, event.GuildID)
}
