package main

import (
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type token struct{}

func stringPtr(x string) *string {
	return &x
}

func interactionUser(event *discordgo.InteractionCreate) (*discordgo.User, error) {
	var user *discordgo.User
	if event.User != nil {
		user = event.User
	} else {
		if event.Member == nil {
			return nil, fmt.Errorf("interaction request recevied without any identified user: %+v", event)
		}
		user = event.Member.User
	}
	return user, nil
}

func username(u *discordgo.User) string {
	if u.Discriminator != "0" {
		return fmt.Sprintf("%s#%s", u.Username, u.Discriminator)
	}
	return u.Username
}
