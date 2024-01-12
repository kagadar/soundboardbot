package soundboard

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"k8s.io/klog/v2"
)

const (
	authUrl        = "https://discord.com/oauth2/authorize?client_id=%s&permissions=%d&scope=bot&disable_guild_select=true&guild_id=%s"
	botPermissions = discordgo.PermissionAdministrator

	createSoundboardCommand      = "create-soundboard"
	createSoundboardSuffixOption = "server_suffix"
)

func (b *bot) initCreateSoundboard() {
	b.commands[createSoundboardCommand] = command{&discordgo.ApplicationCommand{
		Description: "Creates a soundboard and assigns ownership to the calling user",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        createSoundboardSuffixOption,
				Description: "The suffix to add to the created server",
			},
		},
	}, b.createSoundboard}
	b.managerHandlers = append(b.managerHandlers, b.transferServer)
}

func (b *bot) createSoundboard(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	if err := b.validateUser(user, createSoundboardCommand); err != nil {
		return err
	}

	klog.Infof("create guild request received from %q", user)
	var suffix string
	if options[createSoundboardSuffixOption] != nil {
		suffix = " " + strings.Join(strings.Split(strconv.Itoa(int(options[createSoundboardSuffixOption].IntValue())), ""), " ")
	}
	guild, err := b.creator.GuildCreateWithTemplate(b.template, fmt.Sprintf("soundboardhost%s", suffix), "")
	if err != nil {
		mainErr := fmt.Errorf("failed to create guild: %w", err)
		_, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
			Content: toPtr("Failed to create Server"),
		})
		if err != nil {
			mainErr = fmt.Errorf("failed to notify user of error: %w", mainErr)
		}
		return mainErr
	}
	klog.Infof("created guild: %q with default invite channel %q", guild.ID, guild.SystemChannelID)
	if err := b.initialiseDB(ctx, guild); err != nil {
		return err
	}
	klog.Infof("creating invite for %q...", guild.ID)
	invite, doneInvite, err := createInvite(b.creator, &b.creatorInvites, guild, user.ID)
	if err != nil {
		return err
	}
	doneTransfer := make(chan error)
	b.transfers.Store(guild.ID, pendingInvite{user: user.ID, done: doneTransfer})
	if _, err = b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("https://discord.gg/%s", invite.Code)),
	}); err != nil {
		return fmt.Errorf("failed to update follow-up message: %w", err)
	}
	klog.Infof("creator waiting for %q to join %q", user, guild.ID)
	if err := await(ctx, doneInvite); err != nil {
		return fmt.Errorf("%w: failed to wait for %q to join %q", err, user, guild.ID)
	}
	klog.Infof("creator requesting authorization for manager in %q", guild.ID)
	dm, err := b.manager.UserChannelCreate(user.ID)
	if err != nil {
		return fmt.Errorf("failed to open dm with %q: %w", user.ID, err)
	}
	authMsg, err := b.manager.ChannelMessageSend(dm.ID, fmt.Sprintf(authUrl, b.manager.State.Application.ID, botPermissions, guild.ID))
	if err != nil {
		return fmt.Errorf("failed to send dm to %q: %w", user.ID, err)
	}
	klog.Infof("creator waiting for authorization for manager in %q", guild.ID)
	if err := await(ctx, doneTransfer); err != nil {
		return fmt.Errorf("%w: failed to wait for %q to authorize manager in %q", err, user, guild.ID)
	}
	if err := b.manager.ChannelMessageDelete(dm.ID, authMsg.ID); err != nil {
		return fmt.Errorf("failed to delete dm to %q: %w", user.ID, err)
	}
	return nil
}

func (b *bot) transferServer(_ *discordgo.Session, event *discordgo.GuildCreate) {
	// Check if event has a related grant
	grant, ok := b.transfers.Load(event.Guild.ID)
	if !ok {
		return
	}
	if event.OwnerID != b.creator.State.User.ID {
		return
	}
	// Claim the grant
	if _, ok := b.transfers.LoadAndDelete(event.Guild.ID); !ok {
		// Another thread claimed this grant already
		return
	}
	func() {
		b.creator.RLock()
		defer b.creator.RUnlock()
		ok = false
		for _, guild := range b.creator.State.Guilds {
			if guild.ID == event.Guild.ID && guild.OwnerID == b.creator.State.User.ID {
				ok = true
				break
			}
		}
		if !ok {
			return
		}
	}()
	// Claim the grant
	if _, ok := b.transfers.LoadAndDelete(event.Guild.ID); !ok {
		// Another thread claimed this grant already
		return
	}
	var err error
	klog.Infof("manager has joined %q", event.Guild.ID)
	defer func() { grant.done <- err }()
	klog.Infof("moving %q to top priority role in %q", b.manager.State.Application.Name, event.Guild.ID)
	event.Guild.Roles[slices.IndexFunc(event.Guild.Roles, func(r *discordgo.Role) bool {
		return r.Name == b.manager.State.Application.Name
	})].Position = slices.MaxFunc(event.Guild.Roles, func(x, y *discordgo.Role) int {
		return x.Position - y.Position
	}).Position + 1
	if _, err = b.creator.GuildRoleReorder(event.Guild.ID, event.Guild.Roles); err != nil {
		err = fmt.Errorf("failed to reorder roles: %w", err)
		return
	}
	klog.Infof("granting ownership of %q to %q", event.Guild.ID, grant.user)
	if _, err = b.creator.GuildEdit(event.Guild.ID, &discordgo.GuildParams{OwnerID: grant.user}); err != nil {
		err = fmt.Errorf("failed to change guild owner: %w", err)
		return
	}
	if err = b.creator.GuildLeave(event.Guild.ID); err != nil {
		err = fmt.Errorf("failed to leave guild: %w", err)
		return
	}
}
