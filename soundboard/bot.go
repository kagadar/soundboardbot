package soundboard

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"k8s.io/klog/v2"

	"github.com/kagadar/go-set"
	"github.com/kagadar/go-syncmap"
	"github.com/kagadar/soundboardbot/db"
)

var (
	ErrPermissionDenied = errors.New("permission denied")
)

type command struct {
	command *discordgo.ApplicationCommand
	handler func(context.Context, *discordgo.Interaction, *discordgo.User, map[string]*discordgo.ApplicationCommandInteractionDataOption, *discordgo.Message) error
}

type Config struct {
	Admins             []string
	CreatorAccessToken string
	CreatorAppID       discordgo.Snowflake
	ManagerAccessToken string
	ManagerAppId       discordgo.Snowflake
	Template           string
}

type pendingInvite struct {
	user discordgo.Snowflake
	done chan error
}

type bot struct {
	// State
	admins         set.Set[string]
	db             db.DB
	creator        *discordgo.Session
	manager        *discordgo.Session
	creatorInvites syncmap.Map[discordgo.Snowflake, pendingInvite]
	managerInvites syncmap.Map[discordgo.Snowflake, pendingInvite]
	transfers      syncmap.Map[discordgo.Snowflake, pendingInvite]
	roles          set.Set[string]
	template       string

	// Commands and Handlers
	commands        map[string]command
	creatorHandlers []interface{}
	managerHandlers []interface{}
}

type Bot interface {
	Close() error
}

func (b *bot) Close() error {
	return b.manager.Close()
}

func (b *bot) validateUser(user *discordgo.User, command string) error {
	if b.admins.Has(user.Username) {
		return nil
	}
	return fmt.Errorf("%w: %q cannot call %q", ErrPermissionDenied, user, command)
}

func (b *bot) commandler(_ *discordgo.Session, event *discordgo.InteractionCreate) {
	if event.Type != discordgo.InteractionApplicationCommand {
		return
	}
	command, ok := b.commands[event.ApplicationCommandData().Name]
	if !ok {
		klog.Warningf("received command for unexpected interaction type: %q\n%+v", event.ApplicationCommandData().Name, event)
		return
	}
	var user *discordgo.User
	if event.User != nil {
		user = event.User
	} else {
		if event.Member == nil {
			klog.Warningf("interaction request recevied without any identified user: %+v", event)
			return
		}
		user = event.Member.User
	}
	options := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, option := range event.ApplicationCommandData().Options {
		options[option.Name] = option
	}
	if err := b.manager.InteractionRespond(event.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		klog.Errorf("%v: failed to respond to interaction request", err)
		return
	}
	followup, err := b.manager.FollowupMessageCreate(event.Interaction, true, &discordgo.WebhookParams{
		Content: "Working...",
	})
	if err != nil {
		klog.Errorf("%v: failed to send follow-up message", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	if err := command.handler(ctx, event.Interaction, user, options, followup); err != nil {
		klog.Error(err)
		b.manager.FollowupMessageEdit(event.Interaction, followup.ID, &discordgo.WebhookEdit{
			Content: toPtr(err.Error()),
		})
		return
	}
}

func New(config Config, db db.DB) (Bot, error) {
	b := &bot{
		roles:    set.New[string](),
		commands: map[string]command{},
		db:       db,
		admins:   set.New(config.Admins...),
		template: config.Template,
	}

	// Connect to Discord
	var err error
	b.creator, err = discordgo.New(fmt.Sprintf("Bot %s", config.CreatorAccessToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create soundboard session: %w", err)
	}
	b.creator.Identify.Intents |= discordgo.IntentGuildMembers
	if err := b.creator.Open(); err != nil {
		return nil, fmt.Errorf("failed to connect creator to discord: %w", err)
	}
	b.manager, err = discordgo.New(fmt.Sprintf("Bot %s", config.ManagerAccessToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create soundboard session: %w", err)
	}
	b.manager.Identify.Intents |= discordgo.IntentGuildMembers
	if err := b.manager.Open(); err != nil {
		return nil, fmt.Errorf("failed to connect soundboard to discord: %w", err)
	}

	// Load role names from provided template
	template, err := b.manager.GuildTemplate(b.template)
	if err != nil {
		return nil, fmt.Errorf("failed to load template %q: %w", b.template, err)
	}
	for _, role := range template.SerializedSourceGuild.Roles {
		if role.Name != "@everyone" {
			b.roles.Put(role.Name)
		}
	}

	// Attach handlers and application commands
	b.initAddAutorole()
	b.initFixRoles()
	b.initCreateSoundboard()
	b.initDeleteServer()
	b.initInitialiseServer()
	b.initInvite()
	b.initListServers()

	for _, handler := range b.creatorHandlers {
		b.creator.AddHandler(handler)
	}
	b.manager.AddHandler(b.commandler)
	for _, handler := range b.managerHandlers {
		b.manager.AddHandler(handler)
	}
	for name, command := range b.commands {
		command.command.Name = name
		if _, err := b.manager.ApplicationCommandCreate(config.ManagerAppId, "", command.command); err != nil {
			return nil, fmt.Errorf("failed to create application command %q: %w", name, err)
		}
	}
	klog.Infof("Server started as creator:%q manager:%q", b.creator.State.User.ID, b.manager.State.User.ID)
	return b, nil
}
