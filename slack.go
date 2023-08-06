package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/slack-go/slack/socketmode"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"net/http"

	"github.com/akamensky/argparse"
)

func slackBot() {
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
						handleMention(ev) // Deal with commands
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
					handleInteraction(&evt, &callback) // Deal with subsequent interactions from users
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

// Needed to retrieve the given command message
func getLastMessage(channelID string, threadTs string) (lastMessage string, err error) {
	params := slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTs,
	}
	messages, _, _, err := api.GetConversationReplies(&params)
	if err != nil {
		return "", err
	}

	return messages[len(messages)-1].Text, nil
}

func tokenizeCommand(commandMessage string) (tokenizedCommandMessage []string) {
	r := regexp.MustCompile(`\"[^\"]+\"|\S+`)
	tokenizedCommandMessage = r.FindAllString(commandMessage, -1)

	// Oh man this is a bad way to trim these guys
	for index, token := range tokenizedCommandMessage {
		trimmed := strings.Trim(token, `"`)
		//trimmed = strings.Trim(token, `'`) // I dunno why doing this puts the quotes back....
		tokenizedCommandMessage[index] = trimmed
	}

	return tokenizedCommandMessage
}

type Command struct {
	clobber   *bool
	summarize *bool

	title   *string
	section *string

	appendHappened bool
	rangeHappened  bool
	rangeOpts      Range
}
type Range struct {
	firstMessage *string
	lastMessage  *string
}

func interpretCommand(tokenizedCommand []string) (command Command, err error) {
	parser := argparse.NewParser("grab", "A command-line tool for grabbing content")

	command.clobber = parser.Flag("c", "clobber", &argparse.Options{Help: "Overwrite possibly existing content"})
	command.summarize = parser.Flag("s", "summarize", &argparse.Options{Help: "Summarize content"})
	// TODO: Add preview flag for summarizations?

	appendCmd := parser.NewCommand("grab", "Append this thread as new content to the wiki.")
	command.title = appendCmd.StringPositional(&argparse.Options{Required: true, Help: "Title"})
	command.section = appendCmd.StringPositional(&argparse.Options{Required: false, Help: "Section"})

	rangeCmd := parser.NewCommand("range", "Append messages between the given links to the wiki, inclusive")
	command.rangeOpts.firstMessage = rangeCmd.StringPositional(&argparse.Options{Required: true, Help: "First chronological message to be saved"})
	command.rangeOpts.lastMessage = rangeCmd.StringPositional(&argparse.Options{Required: true, Help: "Last chronological message to be saved"})
	rangeTitle := rangeCmd.StringPositional(&argparse.Options{Required: false, Help: "Title"})
	rangeSection := rangeCmd.StringPositional(&argparse.Options{Required: false, Help: "Section"})

	// Redneck's version of "Default Command" because argparse doesn't support default commands
	// If the number of tokens is <= 1 or if the second token is not a recognized keyword/command,
	// then automatically slip "grab" in there
	subCommands := map[string]bool{"grab": true, "range": true, "help": true}
	if len(tokenizedCommand) <= 1 {
		tokenizedCommand = append(tokenizedCommand, "grab")
	} else if !subCommands[tokenizedCommand[1]] {
		tokenizedCommand = append(tokenizedCommand[:2], tokenizedCommand[1:]...)
		tokenizedCommand[1] = "grab"
	}
	
	parser.Parse(tokenizedCommand)

	command.appendHappened = appendCmd.Happened()
	command.rangeHappened = rangeCmd.Happened()
	if rangeCmd.Happened() {
		command.title = rangeTitle
		command.section = rangeSection
	}

	if err != nil {
		return command, err
	}
	return command, nil
}

func rememberCommand(channelID string, threadTs string) (command Command, err error) {
	var lastMessage string
	lastMessage, err = getLastMessage(
		channelID,
		threadTs,
	)
	if err != nil {
		return command, err
	}
	commandMessage := tokenizeCommand(lastMessage)
	command, err = interpretCommand(commandMessage)
	if err != nil {
		return command, err
	}
	return command, nil

}

// Code to run if someone mentions the bot.
func handleMention(ev *slackevents.AppMentionEvent) {
	command, err := interpretCommand(tokenizeCommand(ev.Text))
	if err != nil {
		log.Println(err)
		return
	}

	if command.appendHappened {
		var transcript string

		if *command.title == "" {
			// Get title if not provided
			command.title, transcript = generateTranscript(ev.Channel, ev.ThreadTimeStamp)
		} else {
			_, transcript = generateTranscript(ev.Channel, ev.ThreadTimeStamp)
		}

		// Now that we have the final title, check if the article exists
		newArticleURL, missing, err := getArticleURL(*command.title)
		if err != nil {
			fmt.Println(err)
		}

		sectionExists, _ := sectionExists(*command.title, *command.section)

		// If clobber is set and the page already exists,
		// Send the user a BlockKit form and do nothing else.
		if *(command.clobber) && (!missing || (len(*command.section) > 0 && sectionExists)) {
			warningMessage := fmt.Sprintf("A wiki article with this title already exists! (%s) Are you sure you want to *COMPLETELY OVERWRITE IT?*", newArticleURL)
			confirmButton := slack.NewButtonBlockElement(
				"confirm_wiki_page_overwrite",
				"CONFIRM",
				slack.NewTextBlockObject("plain_text", "CONFIRM", false, false),
			)
			confirmButton.Style = "danger"
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
					confirmButton,
					slack.NewButtonBlockElement(
						"cancel_wiki_page_overwrite",
						"CANCEL",
						slack.NewTextBlockObject("plain_text", "CANCEL", false, false),
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
			return
		}

		// Publish the content to the wiki. If the article doesn't exist,
		// then create it. If the section doesn't exist, then create it.
		err = publishToWiki(!missing, *command.title, *command.section, transcript)
		if err != nil {
			fmt.Println(err)
		}

		// Now that it has been published and definitely exists, get
		// the URL again
		if missing {
			newArticleURL, _, err = getArticleURL(*command.title)
			if err != nil {
				fmt.Println(err)
			}
		}

		// Post ephemeral message to user
		_, err = client.PostEphemeral(ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText(fmt.Sprintf("Article saved! You can find it at: %s", newArticleURL), false))
		if err != nil {
			fmt.Printf("failed posting message: %v", err)
		}
	} else if command.rangeHappened {
	} else {
		// Post ephemeral message to user
		_, err = client.PostEphemeral(ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText("Unrecognized command", false))
		if err != nil {
			fmt.Printf("failed posting message: %v", err)
		}
	}
}

func handleInteraction(evt *socketmode.Event, callback *slack.InteractionCallback) {
	actionID := callback.ActionCallback.BlockActions[0].ActionID
	if actionID == "confirm_wiki_page_overwrite" {
		client.Ack(*evt.Request)

		// We need to get the command given from the transcript. It should
		// be the last message we were asked to get.
		channelID := callback.Container.ChannelID
		threadTs := callback.Container.ThreadTs

		command, err := rememberCommand(
			callback.Container.ChannelID,
			callback.Container.ThreadTs,
		)

		if err != nil {
			log.Println(err)
			return
		}

		// Basically, it's just "handleMention" but without the check for if
		// the article is missing. This one should always just overwrite.
		if command.appendHappened {
			var transcript string

			if *command.title == "" {
				// Get title if not provided
				command.title, transcript = generateTranscript(channelID, threadTs)
			} else {
				_, transcript = generateTranscript(channelID, threadTs)
			}

			// Publish the content to the wiki. If the article doesn't exist,
			// then create it. If the section doesn't exist, then create it.
			err = publishToWiki(false, *command.title, *command.section, transcript)
			if err != nil {
				fmt.Println(err)
			}

			// Update the ephemeral message
			newArticleURL, _, err := getArticleURL(*command.title)
			responseData := fmt.Sprintf(
				`{"replace_original": "true", "thread_ts": "%d", "text": "Article updated! You can find it posted at: %s"}`,
				callback.Container.ThreadTs,
				newArticleURL,
			)
			reader := strings.NewReader(responseData)
			_, err = http.Post(callback.ResponseURL, "application/json", reader)

			if err != nil {
				log.Printf("Failed updating message: %v", err)
			}
		}

	} else if actionID == "cancel_wiki_page_overwrite" {
		client.Ack(*evt.Request) // Tell Slack we got him
		// Update the ephemeral message
		responseData := fmt.Sprintf(
			`{"replace_original": "true", "thread_ts": "%d", "text": "Grab request cancelled."}`,
			callback.Container.ThreadTs,
		)
		reader := strings.NewReader(responseData)
		_, err := http.Post(callback.ResponseURL, "application/json", reader)

		if err != nil {
			log.Printf("Failed updating message: %v", err)
		}
	} else {
		log.Printf("Unexpected Action Occured: %s.\n", actionID, callback.BlockID)
	}
}

// Takes in a slack thread and...
// Gets peoples' CSH usernames and makes them into page links (TODO)
// Removes any mention of Grab
// Adds human readable timestamp to the top of the transcript
// Formats nicely
// Fetches images, uploads them to the Wiki, and links them in appropriately (TODO)
func generateTranscript(channelID string, threadTs string) (title *string, transcript string) {
	// Get the conversation history
	params := slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTs,
	}
	conversation, _, _, err := api.GetConversationReplies(&params)
	if err != nil {
		fmt.Println(err)
	}

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
	conversationUsers := map[string]string{}
	for _, message := range conversation {
		if message.User != authTestResponse.UserID && !strings.Contains(message.Text, fmt.Sprintf("<@%s>", authTestResponse.UserID)) {
			pureConversation = append(pureConversation, message)

			// Translate the user id to a user name
			var msgUser *slack.User
			if len(conversationUsers[message.User]) == 0 {
				msgUser, err = api.GetUserInfo(message.User)
				if err != nil {
					fmt.Println(err)
				} else {
					conversationUsers[message.User] = msgUser.Name
				}
			}
			var msgUserName string
			msgUserName = conversationUsers[message.User]

			transcript += msgUserName + ": " + message.Text + "\n\n"
			fmt.Printf("[%s] %s: %s\n", message.Timestamp, message.User, message.Text)
		}
	}

	return &pureConversation[0].Text, transcript
}
