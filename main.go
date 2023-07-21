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
		panic(err)
	}

	// Log in.
	err = w.Login(config.Username, config.Password)
	if err != nil {
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
						if strings.Contains(ev.Text, "echo") {
							_, _, err := client.PostMessage(ev.Channel, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText("Echo!!", false))
							if err != nil {
								fmt.Printf("failed posting message: %v", err)
							}
						} else if strings.Contains(ev.Text, "page") {
							fmt.Println(ev.Text)
							var split = strings.Split(ev.Text, "page ")
							var pageTitle = split[len(split)-1]
							fmt.Println(pageTitle)
						} else {
							_, err := client.PostEphemeral(ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText("Yes, hello.", false))
							if err != nil {
								fmt.Printf("failed posting message: %v", err)
							}

							// If someone @grab's in a thread, that implies that they want to save the entire contents of the thread.
							// Get every message in the thread, and create a new wiki page with a transcription.

							// Set the options for the API call
							params := slack.GetConversationRepliesParameters{
								ChannelID: ev.Channel,
								Timestamp: ev.ThreadTimeStamp,
							}

							// Get the conversation history
							messages, _, _, err := api.GetConversationReplies(&params)
							if err != nil {
								fmt.Println("Oh fuck that's an error.")
								fmt.Println(err)
							}

							// Print the messages in the conversation history
							var transcript []string
							for _, message := range messages {
								fmt.Printf("[%s] %s: %s\n", message.Timestamp, message.User, message.Text)
								transcript = append(transcript, message.Text)
							}

							// Remove gobbledygook
							//transcript := sanitizeSlackConversation(transcript)

							generateTranscript(messages)

							/*
								// Take what was said and turn it into a paragraph
								convo := strings.Join(transcript, "\n\n")

								err := appendToWiki(string(messages[0].Msg.Text), string(messages[0].Msg.Text), convo)
								if err != nil {
									fmt.Println(err)
								}
							*/
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

					client.Debugf("button clicked!")
				case slack.InteractionTypeShortcut:
				case slack.InteractionTypeViewSubmission:
					// See https://api.slack.com/apis/connections/socket-implement#modal
				case slack.InteractionTypeDialogSubmission:
				default:

				}

				client.Ack(*evt.Request, payload)
			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slack.SlashCommand)
				if !ok {
					fmt.Printf("Ignored %+v\n", evt)

					continue
				}

				client.Debugf("Slash command received: %+v", cmd)

				payload := map[string]interface{}{
					"blocks": []slack.Block{
						slack.NewSectionBlock(
							&slack.TextBlockObject{
								Type: slack.MarkdownType,
								Text: "foo",
							},
							nil,
							slack.NewAccessory(
								slack.NewButtonBlockElement(
									"",
									"somevalue",
									&slack.TextBlockObject{
										Type: slack.PlainTextType,
										Text: "bar",
									},
								),
							),
						),
					},
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
func publishToWiki(title string, convo string) {
	// Push conversation to the wiki, (overwriting whatever was already there, if Grab was the only person to edit?)
	parameters := map[string]string{
		"action": "edit",
		"title":  title,
		"text":   convo,
		"bot":    "true",
	}

	// Make the request.
	err := w.Edit(parameters)
	if err != nil {
		panic(err)
	}
}

// The default behavior, to avoid accidental destructive use.
func appendToWiki(title string, sectionTitle string, convo string) error {
	parameters := map[string]string{
		"action":       "edit",
		"title":        title,
		"section":      "new",
		"sectiontitle": sectionTitle,
		"text":         convo,
		"bot":          "true",
	}

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
func generateTranscript(conversation []slack.Message) (transcript string) {
	// Define the desired format layout
	timeLayout := "2006-01-02 at 15:04"
	currentTime := time.Now().Format(timeLayout)

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
	fmt.Printf("Looking for: <@%s>\n", authTestResponse.UserID)
    var pureConversation []slack.Message
    for _, message := range conversation {
        if message.User != authTestResponse.UserID && !strings.Contains(message.Text, fmt.Sprintf("<@%s>", authTestResponse.UserID)) {
            pureConversation = append(pureConversation, message)
			fmt.Printf("[%s] %s: %s\n", message.Timestamp, message.User, message.Text)
        }
    }

	return transcript
}
