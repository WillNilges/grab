package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"context"

	"github.com/slack-go/slack/socketmode"

	"github.com/slack-go/slack"

	"github.com/joho/godotenv"

	"github.com/EricMCarroll/go-mwclient"
	
	"github.com/go-pg/pg/v10"
)

var config Config

type Config struct {
	WikiURL  string
	Username string
	Password string
	Domain   string
	PostgresURI string
}

var w *mwclient.Client
var api *slack.Client
var client *socketmode.Client

func init() {
	// Load environment variables, one way or another
	err := godotenv.Load()
	if err != nil {
		log.Println("Couldn't load .env file")
	}
	
	// ------- postgres  --------
	
	config.PostgresURI = os.Getenv("POSTGRES_URI")

	opt, err := pg.ParseURL(config.PostgresURI)
	if err != nil {
	   panic(err)
	}

	db := pg.Connect(opt)

	ctx := context.Background()

	if err := db.Ping(ctx); err != nil {
		panic(err)
	}

	// ------- mediawiki --------
	config.WikiURL = os.Getenv("WIKI_URL")
	config.Username = os.Getenv("WIKI_UNAME")
	config.Password = os.Getenv("WIKI_PWORD")
	config.Domain = os.Getenv("WIKI_DOMAIN")

	// Initialize a *Client with New(), specifying the wiki's API URL
	// and your HTTP User-Agent. Try to use a meaningful User-Agent.
	w, err = mwclient.New(config.WikiURL, "Grab")
	if err != nil {
		fmt.Println("Could not create MediaWiki Client instance.")
		panic(err)
	}

	// Log in.
	err = w.Login(config.Username, config.Password)
	if err != nil {
		fmt.Println("Could not log into MediaWiki instance.")
		panic(err)
	}
	// end mediawiki

	// SLACK
	// Get tokens
	appToken := os.Getenv("SLACK_APP_TOKEN")
	if appToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(appToken, "xapp-") {
		fmt.Fprintf(os.Stderr, "SLACK_APP_TOKEN must have the prefix \"xapp-\".")
	}

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if botToken == "" {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN must be set.\n")
		os.Exit(1)
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		fmt.Fprintf(os.Stderr, "SLACK_BOT_TOKEN must have the prefix \"xoxb-\".")
	}

	api = slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(appToken),
	)

	client = socketmode.New(
		api,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)
	// end SLACK
}

func main() {
	slackBot()
}
