package soundboard

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/golang/glog"
	"github.com/kagadar/go-syncmap"

	"github.com/kagadar/soundboardbot/db"
)

type command struct {
	command *discordgo.ApplicationCommand
	handler func(*discordgo.Interaction, *discordgo.User, map[string]*discordgo.ApplicationCommandInteractionDataOption) error
}

type token struct{}

type Config struct {
	AdminRole             string
	SoundboardAccessToken string
	SoundboardAppID       string
	Template              string
}

type usergrant struct {
	user string
	role string
	done chan token
}

type bot struct {
	adminRole     string
	commands      map[string]command
	db            db.DB
	extraHandlers []interface{}
	pendingGrants *syncmap.Map[string, *usergrant]
	soundboard    *discordgo.Session
	template      string
}

type Bot interface {
	Close() error
}

func (b *bot) Close() error {
	return b.soundboard.Close()
}

func (b *bot) commandler(_ *discordgo.Session, event *discordgo.InteractionCreate) {
	if event.Type != discordgo.InteractionApplicationCommand {
		return
	}
	command, ok := b.commands[event.ApplicationCommandData().Name]
	if !ok {
		glog.Warningf("received command for unexpected interaction type: %q\n%+v", event.ApplicationCommandData().Name, event)
		return
	}
	var user *discordgo.User
	if event.User != nil {
		user = event.User
	} else {
		if event.Member == nil {
			glog.Warningf("interaction request recevied without any identified user: %+v", event)
			return
		}
		user = event.Member.User
	}
	options := map[string]*discordgo.ApplicationCommandInteractionDataOption{}
	for _, option := range event.ApplicationCommandData().Options {
		options[option.Name] = option
	}
	if err := command.handler(event.Interaction, user, options); err != nil {
		glog.Error(err)
	}
}

func New(config Config, db db.DB) (Bot, error) {

	b := &bot{adminRole: config.AdminRole, commands: map[string]command{}, db: db, pendingGrants: &syncmap.Map[string, *usergrant]{}, template: config.Template}

	// Init commands
	b.initCreateSoundboard()
	b.initDeleteServer()
	b.initListServers()

	var err error
	b.soundboard, err = discordgo.New(fmt.Sprintf("Bot %s", config.SoundboardAccessToken))
	if err != nil {
		return nil, fmt.Errorf("failed to create soundboard session: %w", err)
	}
	b.soundboard.Identify.Intents |= discordgo.IntentGuildMembers
	if err := b.soundboard.Open(); err != nil {
		return nil, fmt.Errorf("failed to connect soundboard to discord: %w", err)
	}

	for name, command := range b.commands {
		command.command.Name = name
		if _, err := b.soundboard.ApplicationCommandCreate(config.SoundboardAppID, "", command.command); err != nil {
			return nil, fmt.Errorf("failed to create application command %q: %w", name, err)
		}
	}
	b.soundboard.AddHandler(b.commandler)
	for _, handler := range b.extraHandlers {
		b.soundboard.AddHandler(handler)
	}

	glog.Infof("Server started as %q", b.soundboard.State.User.ID)

	return b, nil
}
