package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/kagadar/soundboardbot/db"
	"github.com/kagadar/soundboardbot/soundboard"
)

var (
	adminRole             = flag.String("admin_role_name", "admin", "The name of the admin role used by the provided template")
	soundboardAccessToken = flag.String("soundboard_access_token", "", "Token used by the Soundboard Manager to access Discord")
	soundboardAppID       = flag.String("soundboard_app_id", "1131203534117937182", "The Soundboard Manager's App ID")
	template              = flag.String("soundboard_server_template", "qFRRy4yyx5Da", "The Server Template to use when creating a new soundboard")
	superAdmins           = flag.String("super_admins", "kagadar", "Comma-separated list of bot super admins")
)

func init() { flag.Parse() }

func main() {
	db, err := db.New()
	if err != nil {
		glog.Fatal(err)
	}
	bot, err := soundboard.New(soundboard.Config{
		AdminRole:             *adminRole,
		SoundboardAccessToken: *soundboardAccessToken,
		SoundboardAppID:       *soundboardAppID,
		Template:              *template,
	}, db)
	if err != nil {
		glog.Fatal(err)
	}
	defer bot.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
}
