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
    "github.com/google/uuid"
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
	oldest *string
	latest  *string
}

const helpMessage string = "To grab a thread, ping me, and optionally provide an article title and section title.\nYou can also pass a `-c` flag to OVERWRITE whatever is already on the wiki at the given article/section."

func interpretCommand(tokenizedCommand []string) (command Command, err error) {
	parser := argparse.NewParser("grab", "A command-line tool for grabbing content")

	command.clobber = parser.Flag("c", "clobber", &argparse.Options{Help: "Overwrite possibly existing content"})
	command.summarize = parser.Flag("s", "summarize", &argparse.Options{Help: "Summarize content"})
	// TODO: Add preview flag for summarizations?

	appendCmd := parser.NewCommand("grab", "Append this thread as new content to the wiki.")
	command.title = appendCmd.StringPositional(&argparse.Options{Required: true, Help: "Title"})
	command.section = appendCmd.StringPositional(&argparse.Options{Required: false, Help: "Section"})

	rangeCmd := parser.NewCommand("range", "Append messages between the given links to the wiki, inclusive")
	command.rangeOpts.oldest = rangeCmd.StringPositional(&argparse.Options{Required: true, Help: "First chronological message to be saved"})
	command.rangeOpts.latest = rangeCmd.StringPositional(&argparse.Options{Required: true, Help: "Last chronological message to be saved"})
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
		// Firstly, check if we have a ThreadTimeStamp. If not, scream.
		if ev.ThreadTimeStamp == "" {
			_, err = client.PostEphemeral(
				ev.Channel,
				ev.User,
				slack.MsgOptionTS(ev.ThreadTimeStamp),
				slack.MsgOptionText(
					fmt.Sprintf("Sorry, I only work inside threads!\n%s", helpMessage),
					false,
				),
			)
			if err != nil {
				fmt.Printf("failed posting message: %v", err)
			}
			return
		}

		var transcript string

		conversation, err := getThreadConversation(ev.Channel, ev.ThreadTimeStamp)
		if err != nil {
			log.Println(err)
			return
		}
		if *command.title == "" {
			// Get title if not provided
			command.title, transcript = generateTranscript(conversation)
		} else {
			_, transcript = generateTranscript(conversation)
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
			askToClobber(ev.Channel, ev.User, ev.ThreadTimeStamp, newArticleURL)
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
		// Use given message links to find their timestamps
		// God I hate this fucking language
		fmt.Println(*command.rangeOpts.oldest)
		oldest_url := strings.Split(*command.rangeOpts.oldest, `/`)
		oldest_ts := oldest_url[len(oldest_url)-1]
		index := len(oldest_ts)-1-1-6 // -1 because of the p, -1 because >, -6 because timestamp format
		oldest_ts = oldest_ts[1:index] + "." + oldest_ts[index:len(oldest_ts)-1-1] // Drop the p at position [0], and drop the angle bracket at the end
		fmt.Println(oldest_ts)

		// Get the conversation history
		conversation, err := getConversation(ev.Channel, oldest_ts, "")
		
		for i, j := 0, len(conversation)-1; i < j; i, j = i+1, j-1 {
			conversation[i], conversation[j] = conversation[j], conversation[i]
		}
		if err != nil {
			fmt.Printf("Could not get messages: %v", err)
		}
		var transcript string
		if *command.title == "" {
			// Get title if not provided
			command.title, transcript = generateTranscript(conversation)
		} else {
			_, transcript = generateTranscript(conversation)
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
			askToClobber(ev.Channel, ev.User, ev.TimeStamp, newArticleURL)
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
	} else {
		// Post ephemeral message to user
		_, err = client.PostEphemeral(ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText(helpMessage, false))
		if err != nil {
			fmt.Printf("failed posting message: %v", err)
		}
	}
}

func askToClobber(channel string, user string, timestamp string, newArticleURL string) {
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
		channel,
		user,
		slack.MsgOptionTS(timestamp),
		blockMsg,
	)
	if err != nil {
		log.Printf("Failed to send message: %v", err)
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

			conversation, err := getThreadConversation(channelID, threadTs)
			if err != nil {
				log.Println(err)
				return
			}
			if *command.title == "" {
				// Get title if not provided
				command.title, transcript = generateTranscript(conversation)
			} else {
				_, transcript = generateTranscript(conversation)
			}

			// Publish the content to the wiki. If the article doesn't exist,
			// then create it. If the section doesn't exist, then create it.
			err = publishToWiki(false, *command.title, *command.section, transcript)
			if err != nil {
				log.Println(err)
				return
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

func getConversation(channelID string, oldest string, latest string) (conversation []slack.Message, err error) {
	params := slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Inclusive: true,
		Oldest: oldest,
		Latest: latest,
	}
	var response *slack.GetConversationHistoryResponse
	response, err = api.GetConversationHistory(&params)
	if err != nil {
		return conversation, err
	}
	return response.Messages, nil
}

func getThreadConversation(channelID string, threadTs string) (conversation []slack.Message, err error) {
	// Get the conversation history
	params := slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTs,
	}
	conversation, _, _, err = api.GetConversationReplies(&params)
	if err != nil {
		return conversation, err
	}
	return conversation, nil
}

// Takes in a slack thread and...
// Gets peoples' CSH usernames and makes them into page links (TODO)
// Removes any mention of Grab
// Adds human readable timestamp to the top of the transcript
// Formats nicely
// Fetches images, uploads them to the Wiki, and links them in appropriately (TODO)
func generateTranscript(conversation []slack.Message) (title *string, transcript string) {
	// Define the desired format layout
	timeLayout := "2006-01-02 at 15:04"
	currentTime := time.Now().Format(timeLayout) // FIXME: Wait this is wrong. Should be when the convo begins.

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

		// Don't include messages that mention Grab.
		if message.User == authTestResponse.UserID || strings.Contains(message.Text, fmt.Sprintf("<@%s>", authTestResponse.UserID)) {
			continue
		}
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

		// Check for attachements
		for _, attachment := range message.Attachments {
			fmt.Println("Attachment!!!!")
			// Dead-simple way to grab text attachments.
			if attachment.Text != "" {
				fmt.Println(attachment.Text)
				transcript += "\n\n<pre>" + attachment.Text + "</pre>"
			}
		}

		// I guess files are different.
		for _, file := range message.Files {
			fmt.Println(file.Mimetype)
			fmt.Println(file.URLPrivateDownload)
			// Download the file from Slack
			basename := fmt.Sprintf("%s.%s", uuid.New(), file.Filetype)
			path := fmt.Sprintf("/tmp/%s", basename)
			tempFile, err := os.Create(path)
			defer os.Remove(path)
			if err != nil {
				fmt.Println("Error creating output file:", err)
				return
			}
			err = client.GetFile(file.URLPrivateDownload, tempFile)
			if err != nil {
				log.Println("Error getting file from Slack: ", err)
				return
			}
			fmt.Printf("File created at %s\n", path)
			tempFile.Close()
			/*
				Check the file type.
				If it's an image, then check the File ID. Create a file in /tmp or
				something, download it, then upload it to MediaWiki.
			*/
			if strings.Contains(file.Mimetype, "image") {
				// Upload it to MediaWiki. For some reason, I can't just re-use
				// the file header. The API doesn't like it.
				var fileTitle string
				fileTitle, err = uploadToWiki(path)
				if err != nil {
					log.Println("Error uploading file: ", err)
					return
				}
				// It'll be like uhhh [[File:name.jpg]] or whatever.
				transcript += fmt.Sprintf("[[File:%s]]\n\n", fileTitle)
			} else if strings.Contains(file.Mimetype, "text") {
				fileContents, err := os.ReadFile(path)
				if err != nil {
					log.Println("Error reading file: ", err)
					return
				}
				transcript += file.Name + ":\n<pre>" + string(fileContents) + "</pre>\n\n"
			}
		}
	}

	return &pureConversation[0].Text, transcript
}

