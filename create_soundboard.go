package main

import (
	"flag"
	"fmt"
	"soundboardbot/syncmap"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
)

type usergrant struct {
	user string
	role string
	done chan token
}

const (
	createSoundboardNameOption = "server_suffix"
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
				Name:        createSoundboardNameOption,
				Description: "The suffix to add to the created server",
			},
		},
	}, createSoundboard}
	extraHandlers = append(extraHandlers, grantOwnership)
}

func createSoundboard(s *discordgo.Session, event *discordgo.InteractionCreate) {
	user, err := interactionUser(event)
	if err != nil {
		glog.Error(err)
		return
	}
	username := username(user)
	glog.Infof("create guild request received from %q", username)

	if err := s.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		glog.Errorf("failed to respond to interaction request: %v", err)
		return
	}
	msg, err := s.FollowupMessageCreate(event.Interaction, true, &discordgo.WebhookParams{
		Content: "Working...",
	})
	if err != nil {
		glog.Errorf("failed to send follow-up message: %v", err)
		return
	}

	guild, err := s.GuildCreateWithTemplate(*template, "soundboardhost", "")
	if err != nil {
		glog.Errorf("failed to create guild: %v", err)
		_, err := s.FollowupMessageEdit(event.Interaction, msg.ID, &discordgo.WebhookEdit{
			Content: stringPtr("Failed to create Server"),
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

	_, err = s.FollowupMessageEdit(event.Interaction, msg.ID, &discordgo.WebhookEdit{
		Content: stringPtr(fmt.Sprintf("https://discord.gg/%s", invite.Code)),
	})
	if err != nil {
		glog.Errorf("failed to update follow-up message: %v", err)
		return
	}

	glog.Infof("waiting for %q to take over %q", username, guild.ID)
	<-done

	glog.Infof("leaving guild %q", guild.ID)

	if err := s.GuildLeave(guild.ID); err != nil {
		glog.Errorf("failed to leave guild: %v", err)
		return
	}
	glog.Infof("left guild %q", guild.ID)

	if err := s.FollowupMessageDelete(event.Interaction, msg.ID); err != nil {
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

	username := username(event.User)

	glog.Infof("%q has joined %q, granting ownership", username, event.GuildID)
	defer func() { grant.done <- token{} }()

	if err := s.GuildMemberRoleAdd(event.GuildID, grant.user, grant.role); err != nil {
		glog.Errorf("failed to grant role to user: %v", err)
	}
	if _, err := s.GuildEdit(event.GuildID, &discordgo.GuildParams{OwnerID: grant.user}); err != nil {
		glog.Errorf("failed to change guild owner: %v", err)
	}
	glog.Infof("%q has taken ownership of %q", username, event.GuildID)
}
