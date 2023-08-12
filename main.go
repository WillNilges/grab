package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/slack-go/slack"

	"github.com/joho/godotenv"

	"github.com/EricMCarroll/go-mwclient"

	"database/sql"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/gin-gonic/gin"
)

var config Config

type Config struct {
	WikiURL     string
	Username    string
	Password    string
	Domain      string
	PostgresURI string
}

var w *mwclient.Client
var api *slack.Client

// var client *socketmode.Client
var db *bun.DB

func init() {
	// Load environment variables, one way or another
	err := godotenv.Load()
	if err != nil {
		log.Println("Couldn't load .env file")
	}

	// ------- postgres  --------
	dsn := os.Getenv("POSTGRES_URI")
	pgdb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	// Create a Bun db on top of it.
	db = bun.NewDB(pgdb, pgdialect.New())

	err = initDB(db)
	if err != nil {
		log.Fatal(err)
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

}

func main() {
	app := gin.Default()
	app.Any("/install/authorize", installResp())
	app.GET("/install/form", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})
	app.GET("/install/submit", func(c *gin.Context) {
		wikiUsername := c.PostForm("username")
		wikiPassword := c.PostForm("password")
		wikiUrl := c.PostForm("url")

		// You can process the data here (e.g., save to a database, perform validation)
		c.Redirect(http.StatusSeeOther, "/install/authorize?username="+wikiUsername+"&password="+wikiPassword+"&url="+wikiUrl)
	})

	// Serve initial interactions with the bot
	eventGroup := app.Group("/event")
	//eventGroup.Use(signatureVerification)
	eventGroup.POST("/handle", eventResp())
	// eventGroup.POST("/grab", appendResp())
	// eventGroup.POST("/range", rangeResp())

	interactionGroup := app.Group("/interaction")
	interactionGroup.POST("/handle", interactionResp())

	_ = app.Run()
}
