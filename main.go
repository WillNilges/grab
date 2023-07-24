package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/slack-go/slack/socketmode"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/joho/godotenv"

	"github.com/EricMCarroll/go-mwclient"
)

var config Config

type Config struct {
	WikiURL  string
	Username string
	Password string
	Domain   string
}

var w *mwclient.Client
var api *slack.Client
var client *socketmode.Client

func init() {
	// Load environment variables, one way or another
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
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
	go func() {
		for evt := range client.Events {
			switch evt.Type {
			case socketmode.EventTypeHello:
				fmt.Println("Greetings!")
			case socketmode.EventTypeConnecting:
				fmt.Println("Connecting to Slack with Socket Mode...")
			case socketmode.EventTypeConnectionError:
				fmt.Println("Connection failed. Retrying later...")
			case socketmode.EventTypeConnected:
				fmt.Println("Connected to Slack with Socket Mode.")
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				fmt.Printf("Event received: %+v\n", eventsAPIEvent)

				client.Ack(*evt.Request)

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						commandMessage := strings.Split(ev.Text, " ")
						var subCommand string
						if len(commandMessage) >= 2 {
							subCommand = commandMessage[1]
						}
						if strings.Contains(subCommand, "append") {
						} else if strings.Contains(subCommand, "range") {
						} else if strings.Contains(subCommand, "help") {
							
						} else { // Default behavior
							// If someone @grab's in a thread, that implies that they want to save the entire contents of the thread.
							// Get every message in the thread, and create a new wiki page with a transcription.

							// Check if user provided a title
							var possibleTitle string
							if len(commandMessage) > 1 {
								possibleTitle = strings.Join(commandMessage[1:], " ")
							}

							// Check if a page with the same title exists

							// Get the conversation history
							params := slack.GetConversationRepliesParameters{
								ChannelID: ev.Channel,
								Timestamp: ev.ThreadTimeStamp,
							}
							messages, _, _, err := api.GetConversationReplies(&params)
							if err != nil {
								fmt.Println("Oh fuck that's an error.")
								fmt.Println(err)
							}

							// Print the messages in the conversation history
							var transcript string
							if len(possibleTitle) > 0 {
								_, transcript = generateTranscript(messages)
							} else {
								possibleTitle, transcript = generateTranscript(messages)
							}

							// Now that we have the final title, check if the article exists
							newArticleURL, missing, err := getArticleURL(possibleTitle)

							if !missing {
								// Scream at user
								/*_, err = client.PostEphemeral( ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText( "A wiki article already exists!", false))
								if err != nil {
									fmt.Printf("failed posting message: %v", err)
								}
								break*/

								// Create a block kit message to ask the user
								// if they _really_ want to overwrite the page
								warningMessage := fmt.Sprintf("A wiki article with this title already exists! (%s) Are you sure you want to *COMPLETELY OVERWRITE IT?*", newArticleURL)
								blockMsg := slack.MsgOptionBlocks(
									slack.NewSectionBlock(
										slack.NewTextBlockObject(
											"mrkdwn",
											warningMessage, 
											false,
											false,
										),
										nil,
										nil,
									),
									slack.NewActionBlock(
										"",
										slack.NewButtonBlockElement(
											"confirm_wiki_page_overwrite",
											"CONFIRM",
											slack.NewTextBlockObject("plain_text", "CONFIRM", false, false),
										),
									),
								)

								_, err := api.PostEphemeral(
									ev.Channel, 
									ev.User, 
									slack.MsgOptionTS(ev.ThreadTimeStamp), 
									blockMsg,
								)
								if err != nil {
									log.Printf("Failed to send message: %v", err)
								}
							} else {
								err = publishToWiki(possibleTitle, transcript)
								if err != nil {
									fmt.Println(err)
								}

								baseResponse := "Article saved! You can find it posted at: "	
								newArticleURL, missing, err = getArticleURL(possibleTitle)
								if err != nil {
									fmt.Println(err)
								}

								// Post ephemeral message to user
								_, err = client.PostEphemeral( ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText( fmt.Sprintf("%s %s", baseResponse, newArticleURL), false))
								if err != nil {
									fmt.Printf("failed posting message: %v", err)
								}
							}
						}
					case *slackevents.MemberJoinedChannelEvent:
						fmt.Printf("user %q joined to channel %q", ev.User, ev.Channel)
					}
				default:
					client.Debugf("unsupported Events API event received")
				}
			case socketmode.EventTypeInteractive:
				callback, ok := evt.Data.(slack.InteractionCallback)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				fmt.Printf("Interaction received: %+v\n", callback)

				var payload interface{}

				switch callback.Type {
				case slack.InteractionTypeBlockActions:
					// See https://api.slack.com/apis/connections/socket-implement#button
					actionID := callback.ActionCallback.BlockActions[0].ActionID
					if actionID == "confirm_wiki_page_overwrite" {
						client.Ack(*evt.Request)

						// First, delete the old message (fuck you too, slack)
						api.DeleteMessage(callback.Channel.ID, callback.Message.Timestamp)
						
						_, err := client.PostEphemeral(callback.Channel.ID, callback.User.ID, slack.MsgOptionTS(callback.OriginalMessage.ThreadTimestamp), slack.MsgOptionText("I will save it if you implement that function ;)", false))

						if err != nil {
							log.Printf("Failed updating message: %v", err)
						}
					} else {
						log.Printf("Unexpected Action Occured: %s.\n", actionID, callback.BlockID)
					}
				default:

				}

				client.Ack(*evt.Request, payload)
			default:
				fmt.Fprintf(os.Stderr, "Unexpected event type received: %s\n", evt.Type)
			}
		}
	}()

	client.Run()
}

// Simply takes a title and a string, and puts it on the wiki, clobbering
// whatever used to be there
func publishToWiki(title string, convo string) (err error) {
	// Push conversation to the wiki, (overwriting whatever was already there, if Grab was the only person to edit?)
	parameters := map[string]string{
		"action": "edit",
		"title":  title,
		"text":   convo,
		"bot":    "true",
	}

	// Make the request.
	err = w.Edit(parameters)
	if err != nil {
		return err	
	}
	return nil
}

// The default behavior, to avoid accidental destructive use.
func appendToWiki(title string, sectionTitle string, convo string) error {
	parameters := map[string]string{
		"action":       "edit",
		"title":        title,
		"section":      "new",
		"sectiontitle": sectionTitle,
		"appendtext":   convo,
		"bot":          "true",
	}

	fmt.Println(parameters)

	// Make the request.
	err := w.Edit(parameters)
	return err
}

// Takes in a slack thread and...
// Gets peoples' CSH usernames and makes them into page links (TODO)
// Removes any mention of Grab (TODO)
// Adds human readable timestamp to the top of the transcript (TODO)
// Formats nicely
// Fetches images, uploads them to the Wiki, and links them in appropriately (TODO)
func generateTranscript(conversation []slack.Message) (title string, transcript string) {
	// Define the desired format layout
	timeLayout := "2006-01-02 at 15:04"
	currentTime := time.Now().Format(timeLayout)

	transcript += "Conversation begins at " + currentTime + "\n\n"

	// Remove any message sent by Grab
	// Call the AuthTest method to check the authentication and retrieve the bot's user ID
	authTestResponse, err := api.AuthTest()
	if err != nil {
		log.Fatalf("Error calling AuthTest: %s", err)
	}

	// Print the bot's user ID
	fmt.Printf("Current Time: %s\n", currentTime)
	fmt.Printf("Bot UserID: %s\n", authTestResponse.UserID)

	// Remove messages sent by Grab	and mentioning Grab
	// Format conversation into string line-by-line
	fmt.Printf("Looking for: <@%s>\n", authTestResponse.UserID)
    var pureConversation []slack.Message
    for _, message := range conversation {
        if message.User != authTestResponse.UserID && !strings.Contains(message.Text, fmt.Sprintf("<@%s>", authTestResponse.UserID)) {
            pureConversation = append(pureConversation, message)
			transcript += message.User + ": " + message.Text + "\n\n"
			fmt.Printf("[%s] %s: %s\n", message.Timestamp, message.User, message.Text)
        }
    }


	return pureConversation[0].Text, transcript
}

func getArticleURL(title string) (url string, missing bool, err error) {
	newArticleParameters := map[string]string {
		"action": "query",
		"format": "json",
		"titles": title,
		"prop": "info",
		"inprop": "url",
	}

	newArticle, err := w.Get(newArticleParameters)

	fmt.Println("CHECKING FOR ARTICLE")
	fmt.Println(newArticle)

	if err != nil {
		return "", false, err
	}

	pages, err := newArticle.GetObjectArray("query", "pages")
	for _, page := range pages {
		url, err = page.GetString("canonicalurl")
		missing, _ = page.GetBoolean("missing")
		break // Just get first one. There won't ever not be just one.
	}

	if err != nil {
		return "", false, err
	}

	return url, missing, nil
}
