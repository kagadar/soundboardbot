package soundboard

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/kagadar/go-set"
	"k8s.io/klog/v2"
)

const (
	initialiseDBCommand       = "initialise-db"
	initialiseDBOptionAll     = "all"
	initialiseDBOptionCurrent = "current"
)

func (b *bot) initInitialiseServer() {
	b.commands[initialiseDBCommand] = command{&discordgo.ApplicationCommand{
		Description: "Initialises the bot's DB entries for soundboard servers",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        initialiseDBOptionAll,
				Description: "Reinitialise all servers that the bot is a member of",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        initialiseDBOptionCurrent,
				Description: "Reinitialise the server that this command is executing in",
			},
		},
	}, b.initialiseDBCommand}
}

func (b *bot) checkRoleOrder(guild *discordgo.Guild, appName string) bool {
	return slices.MaxFunc(guild.Roles, func(x, y *discordgo.Role) int { return x.Position - y.Position }).Name == appName
}

func (b *bot) initialiseDB(ctx context.Context, guild *discordgo.Guild) error {
	roles := map[string]discordgo.Snowflake{}
	for _, role := range guild.Roles {
		if b.roles.Has(role.Name) {
			roles[role.Name] = role.ID
		}
	}
	if err := b.db.UpsertSoundboard(ctx, guild.ID, roles); err != nil {
		return err
	}
	return nil
}

func (b *bot) initialiseDBCommand(ctx context.Context, interaction *discordgo.Interaction, user *discordgo.User, options map[string]*discordgo.ApplicationCommandInteractionDataOption, followup *discordgo.Message) error {
	if err := b.validateUser(user, initialiseDBCommand); err != nil {
		return err
	}
	klog.Infof("initialise db request received from %q", user)
	invalidPriority := set.New[string]()
	for opt := range options {
		switch opt {
		case initialiseDBOptionAll:
			klog.Info("initialising db for all guilds")
			unmanagedGuilds, err := b.db.ListGuilds(ctx)
			if err != nil {
				return err
			}
			staleData, err := b.db.ListSoundboards(ctx)
			if err != nil {
				return err
			}
			ugs, err := queryAll(func(last *discordgo.UserGuild) ([]*discordgo.UserGuild, error) {
				return b.manager.UserGuilds(200, "", last.ID)
			})
			if err != nil {
				return err
			}
			for _, ug := range ugs {
				guild, err := b.manager.Guild(ug.ID)
				if err != nil {
					return err
				}
				if unmanagedGuilds.Has(guild.ID) {
					continue
				}
				if staleData.Has(guild.ID) {
					delete(staleData, guild.ID)
				}
				klog.Infof("initialising db for %q(%s)", guild.Name, guild.ID)
				if err := b.initialiseDB(ctx, guild); err != nil {
					return err
				}
				if !b.checkRoleOrder(guild, b.manager.State.User.Username) {
					invalidPriority.Put(guild.Name)
				}
			}
			for guildID := range staleData {
				klog.Infof("deleting stale db entry %q", guildID)
				if err := b.db.DeleteSoundboard(ctx, guildID); err != nil {
					return err
				}
			}
		case initialiseDBOptionCurrent:
			klog.Infof("initialising db for %q only", interaction.GuildID)
			guild, err := b.manager.Guild(interaction.GuildID)
			if err != nil {
				return fmt.Errorf("%w: failed to lookup guild %q", err, interaction.GuildID)
			}
			if err := b.initialiseDB(ctx, guild); err != nil {
				return err
			}
			if !b.checkRoleOrder(guild, b.manager.State.User.Username) {
				invalidPriority.Put(string(guild.Name))
			}
		}
	}
	if _, err := b.manager.FollowupMessageEdit(interaction, followup.ID, &discordgo.WebhookEdit{
		Content: toPtr(fmt.Sprintf("db has been initialised\nThe following guilds need manual role reordering:\n\t%s", strings.Join(invalidPriority.Elements(), "\n\t"))),
	}); err != nil {
		return fmt.Errorf("%w: failed to notify %q of completed initialise DB request", err, user)
	}
	klog.Infof("db has been initialised by %q", user)
	return nil
}
