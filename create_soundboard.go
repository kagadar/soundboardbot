package main

import (
	"flag"
	"fmt"
	"soundboardbot/syncmap"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

type usergrant struct {
	user string
	role string
	done chan token
}

const (
	createSoundboardSuffixOption = "server_suffix"
)

var (
	template      = flag.String("soundboard_server_template", "qFRRy4yyx5Da", "The Server Template to use when creating a new soundboard")
	adminRoleName = flag.String("admin_role_name", "admin", "The name of the admin role used by the provided template")

	pendingGrants = &syncmap.Map[string, *usergrant]{}
)

func init() {
	commands["create-soundboard"] = command{&discordgo.ApplicationCommand{
		Description: "Creates a soundboard and assigns ownership to the calling user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        createSoundboardSuffixOption,
				Description: "The suffix to add to the created server",
			},
		},
	}, createSoundboard}
	extraHandlers = append(extraHandlers, grantOwnership)
}

func createSoundboard(s *discordgo.Session, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption) {
	glog.Infof("create guild request received from %q", user)

	if err := s.InteractionRespond(interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		glog.Errorf("failed to respond to interaction request: %v", err)
		return
	}
	msg, err := s.FollowupMessageCreate(interaction, true, &discordgo.WebhookParams{
		Content: "Working...",
	})
	if err != nil {
		glog.Errorf("failed to send follow-up message: %v", err)
		return
	}
	var suffix string
	if suffixValue := int(options[createSoundboardSuffixOption].IntValue()); suffixValue > 0 {
		suffix = " " + strings.Join(strings.Split(strconv.Itoa(suffixValue), ""), " ")
	}
	guild, err := s.GuildCreateWithTemplate(*template, fmt.Sprintf("soundboardhost%s", suffix), "")
	if err != nil {
		glog.Errorf("failed to create guild: %v", err)
		_, err := s.FollowupMessageEdit(interaction, msg.ID, &discordgo.WebhookEdit{
			Content: toPtr("Failed to create Server"),
		})
		if err != nil {
			glog.Errorf("failed to notify user of error: %v", err)
		}
		return
	}
	glog.Infof("created guild: %q\ndefault invite channel is %q", guild.ID, guild.SystemChannelID)
	glog.Infof("finding admin role for %q...", guild.ID)
	var admin *discordgo.Role
	for _, role := range guild.Roles {
		if role.Name == *adminRoleName {
			admin = role
			glog.Infof("found admin role in %q: %q", guild.ID, role.ID)
		}
	}
	if admin == nil {
		glog.Errorf("no admin role found in %q", guild.ID)
		return
	}
	done := make(chan token)
	pendingGrants.Store(guild.ID, &usergrant{user: user.ID, role: admin.ID, done: done})
	invite, err := s.ChannelInviteCreate(guild.SystemChannelID, discordgo.Invite{})
	if err != nil {
		glog.Errorf("failed to create guild invite: %v", err)
		return
	}

	_, err = s.FollowupMessageEdit(interaction, msg.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("https://discord.gg/%s", invite.Code)),
	})
	if err != nil {
		glog.Errorf("failed to update follow-up message: %v", err)
		return
	}

	glog.Infof("waiting for %q to take over %q", user, guild.ID)
	<-done

	glog.Infof("leaving guild %q", guild.ID)

	if err := s.GuildLeave(guild.ID); err != nil {
		glog.Errorf("failed to leave guild: %v", err)
		return
	}
	glog.Infof("left guild %q", guild.ID)

	if err := s.FollowupMessageDelete(interaction, msg.ID); err != nil {
		glog.Errorf("failed to delete message: %v", err)
		return
	}
}

func grantOwnership(s *discordgo.Session, event *discordgo.GuildMemberAdd) {
	grant, ok := pendingGrants.Load(event.GuildID)
	if !ok {
		return
	}
	if grant.user != event.User.ID {
		return
	}

	glog.Infof("%q has joined %q, granting ownership", event.User, event.GuildID)
	defer func() { grant.done <- token{} }()

	if err := s.GuildMemberRoleAdd(event.GuildID, grant.user, grant.role); err != nil {
		glog.Errorf("failed to grant role to user: %v", err)
	}
	if _, err := s.GuildEdit(event.GuildID, &discordgo.GuildParams{OwnerID: grant.user}); err != nil {
		glog.Errorf("failed to change guild owner: %v", err)
	}
	glog.Infof("%q has taken ownership of %q", event.User, event.GuildID)
}
