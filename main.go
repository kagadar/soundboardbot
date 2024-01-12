package main

import (
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/kagadar/soundboardbot/db"
	"github.com/kagadar/soundboardbot/soundboard"
	"k8s.io/klog/v2"
)

var (
	admins             = flag.String("admins", "kagadar", "Comma-separated list of bot admins")
	creatorAccessToken = flag.String("creator_access_token", "", "Token used by Creator to access Discord")
	creatorAppID       = flag.String("creator_app_id", "1132277255410831360", "The Creator's App ID")
	managerAccessToken = flag.String("manager_access_token", "", "Token used by the Soundboard Manager to access Discord")
	managerAppID       = flag.String("manager_app_id", "1131203534117937182", "The Soundboard Manager's App ID")
	template           = flag.String("soundboard_server_template", "qFRRy4yyx5Da", "The Server Template to use when creating a new soundboard")
)

func init() {
	klog.InitFlags(nil)
	flag.Parse()
}

func main() {
	db, err := db.New()
	if err != nil {
		klog.Fatal(err)
	}
	bot, err := soundboard.New(soundboard.Config{
		Admins:             strings.Split(strings.ReplaceAll(*admins, " ", ""), ","),
		CreatorAccessToken: *creatorAccessToken,
		CreatorAppID:       discordgo.Snowflake(*creatorAppID),
		ManagerAccessToken: *managerAccessToken,
		ManagerAppId:       discordgo.Snowflake(*managerAppID),
		Template:           *template,
	}, db)
	if err != nil {
		klog.Fatal(err)
	}
	defer bot.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
