package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/EricMCarroll/go-mwclient"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/google/uuid"
)

type GrabCallbackIDs string
type GrabBlockActionIDs string

const (
	// Callback ID
	GrabInteractionAppendThreadTranscript = "append_thread_transcript"
	// Block Action IDs for that Callback ID
	GrabInteractionAppendThreadTranscriptConfirm = "append_thread_transcript_confirm"
	GrabInteractionAppendThreadTranscriptCancel  = "append_thread_transcript_cancel"
)

func signatureVerification(c *gin.Context) {
	verifier, err := slack.NewSecretsVerifier(c.Request.Header, os.Getenv("SIGNATURE_SECRET"))
	if err != nil {
		c.String(http.StatusBadRequest, "error initializing signature verifier: %s", err.Error())
		return
	}
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.String(http.StatusInternalServerError, "error reading request body: %s", err.Error())
		return
	}
	bodyBytesCopy := make([]byte, len(bodyBytes))
	copy(bodyBytesCopy, bodyBytes)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytesCopy))
	if _, err = verifier.Write(bodyBytes); err != nil {
		c.String(http.StatusInternalServerError, "error writing request body bytes for verification: %s", err.Error())
		return
	}
	if err = verifier.Ensure(); err != nil {
		c.String(http.StatusUnauthorized, "error verifying slack signature: %s", err.Error())
		return
	}
	c.Next()
}

func installResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		_, errExists := c.GetQuery("error")
		if errExists {
			c.String(http.StatusOK, "error installing app")
			return
		}
		code, codeExists := c.GetQuery("code")
		if !codeExists {
			c.String(http.StatusBadRequest, "missing mandatory 'code' query parameter")
			return
		}
		resp, err := slack.GetOAuthV2Response(http.DefaultClient,
			os.Getenv("SLACK_CLIENT_ID"),
			os.Getenv("SLACK_CLIENT_SECRET"),
			code,
			"")
		if err != nil {
			c.String(http.StatusInternalServerError, "error exchanging temporary code for access token: %s", err.Error())
			return
		}

		instance := new(Instance)
		if err != nil {
			c.String(http.StatusInternalServerError, "error storing slack access token: %s", err.Error())
			return
		}
		instance.GrabID = uuid.New().String()
		instance.SlackTeamID = resp.Team.ID
		instance.SlackAccessToken = resp.AccessToken
		instance.MediaWikiUname = c.Query("mediaWikiUname")
		instance.MediaWikiPword = c.Query("mediaWikiPword")
		instance.MediaWikiURL = c.Query("mediaWikiURL")

		err = insertInstance(db, instance)
		if err != nil {
			c.String(http.StatusInternalServerError, "error storing slack access token: %s", err.Error())
			return
		}
		c.Redirect(http.StatusFound, fmt.Sprintf("slack://app?team=%s&id=%s&tab=about", resp.Team.ID, resp.AppID))
	}
}

func eventResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading slack event payload: %s", err.Error())
			return
		}
		event, err := slackevents.ParseEvent(bodyBytes, slackevents.OptionNoVerifyToken())
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading slack event payload: %s", err.Error())
			return
		}
		switch event.Type {
		case slackevents.URLVerification:
			ve, ok := event.Data.(*slackevents.EventsAPIURLVerificationEvent)
			if !ok {
				c.String(http.StatusBadRequest, "invalid url verification event payload sent from slack")
				return
			}
			c.JSON(http.StatusOK, &slackevents.ChallengeResponse{
				Challenge: ve.Challenge,
			})
		case slackevents.AppRateLimited:
			c.String(http.StatusOK, "ack")
		case slackevents.CallbackEvent:
			ce, ok := event.Data.(*slackevents.EventsAPICallbackEvent)
			if !ok {
				c.String(http.StatusBadRequest, "invalid callback event payload sent from slack")
				return
			}
			ie := &slackevents.EventsAPIInnerEvent{}
			if err = json.Unmarshal(*ce.InnerEvent, ie); err != nil {
				c.String(http.StatusBadRequest, "invalid inner event payload sent from slack: %s", err.Error())
				return
			}
			switch ie.Type {
			case string(slackevents.AppMention):
				log.Println("Got mentioned")
				am := &slackevents.AppMentionEvent{}
				json.Unmarshal(*ce.InnerEvent, am)
				err := handleMention(ce, am)
				if err != nil {
					log.Println("Error responding to thread: ", err)
					c.String(http.StatusInternalServerError, "Could not respond to thread: %s", err.Error())
				}
			case string(slackevents.AppUninstalled):
				log.Printf("App uninstalled from %s.\n", event.TeamID)
				err = deleteInstance(db, event.TeamID)
				if err != nil {
					c.String(http.StatusInternalServerError, "error handling app uninstallation")
				}
			default:
				c.String(http.StatusBadRequest, "no handler for event of given type")
			}
		default:
			c.String(http.StatusBadRequest, "invalid event type sent from slack")
		}
	}
}

// TODO: Do we really need this?
func handleMention(ce *slackevents.EventsAPICallbackEvent, am *slackevents.AppMentionEvent) (err error) {
	// Retrieve credentials to log into Slack and MediaWiki
	var instance Instance
	instance, err = selectInstanceByTeamID(db, ce.TeamID)
	if err != nil {
		return err
	}
	slackClient := slack.New(instance.SlackAccessToken)

	_, err = slackClient.PostEphemeral(
		am.Channel,
		am.User,
		slack.MsgOptionTS(am.ThreadTimeStamp),
		slack.MsgOptionText("Hello, I'm Grab! A bot that can transcribe Slack threads to your knowledge base.\n\nTo use me, select my shortcut from the dropdown on a threaded message.", false),
	)
	if err != nil {
		return err
	}
	return nil
}

/*
type GrabCommand struct {
	ChatClient ChatClient
	WikiClient WikiClient
}

type Thread struct {
	Title string
	Text string
}

type ChatClient interface {
	processThread() Thread
}

type WikiClient interface {
	publishThread(thread Thread)
}
*/

func generateModalRequest(channelID string, threadTS string) slack.ModalViewRequest {
	// Create a ModalViewRequest with a header and two inputs
	titleText := slack.NewTextBlockObject("plain_text", "Grab a thread", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "Cancel", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "Submit", false, false)

	// === TEXT BLOCK AT THE TOP OF MESSAGE ===
	savingMessage := "Saving thread transcript! Please provide some article info. You can specify existing articles and sections, or come up with new ones."
	messageText := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", savingMessage, false, false), nil, nil,
	)

	// Article Title
	articleTitleText := slack.NewTextBlockObject("plain_text", "Enter Article Title", false, false)
	articleTitlePlaceholder := slack.NewTextBlockObject("plain_text", "Article Title", false, false)
	articleTitleElement := slack.NewPlainTextInputBlockElement(articleTitlePlaceholder, "articleTitle")
	articleTitle := slack.NewInputBlock("Article Title", articleTitleText, nil, articleTitleElement)

	// Section title
	sectionTitleText := slack.NewTextBlockObject("plain_text", "Enter Section Title", false, false)
	sectionTitlePlaceholder := slack.NewTextBlockObject("plain_text", "Section Title", false, false)
	sectionTitleElement := slack.NewPlainTextInputBlockElement(sectionTitlePlaceholder, "sectionTitle")
	sectionTitle := slack.NewInputBlock("Section Title", sectionTitleText, nil, sectionTitleElement)

	// The checkbox. Why I need like 6 fucking lines for this is beyond me.
	clobberCheckboxOptionText := slack.NewTextBlockObject(
		"plain_text", "Overwrite existing content", false, false,
	)
	clobberWarning := "By selecting this, any data already present under the provided article/section will be ERASED."
	clobberCheckboxDescriptionText := slack.NewTextBlockObject("plain_text", clobberWarning, false, false)
	clobberCheckbox := slack.NewCheckboxGroupsBlockElement(
		"clobber",
		slack.NewOptionBlockObject("confirmed", clobberCheckboxOptionText, clobberCheckboxDescriptionText),
	)
	clobberBox := slack.NewInputBlock(
		"Clobber", slack.NewTextBlockObject(slack.PlainTextType, " ", false, false), nil, clobberCheckbox,
	)
	clobberBox.Optional = true // People shouldn't write go

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			messageText,
			articleTitle,
			sectionTitle,
			clobberBox,
		},
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = slack.ViewType("modal")
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	modalRequest.ExternalID = fmt.Sprintf("%s,%s", channelID, threadTS)
	fmt.Println("Chom")
	fmt.Println(modalRequest.ExternalID)
	return modalRequest
}

func interactionResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		fmt.Println(c.Request.FormValue("payload"))
		var payload slack.InteractionCallback
		err := json.Unmarshal([]byte(c.Request.FormValue("payload")), &payload)
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading slack interaction payload: %s", err.Error())
			return
		}

		// Retrieve credentials to log into Slack and MediaWiki
		var instance Instance
		instance, err = selectInstanceByTeamID(db, payload.User.TeamID)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "error reading slack access token: %s", err.Error())
		}
		slackClient := slack.New(instance.SlackAccessToken)

		w, err := mwclient.New(instance.MediaWikiURL, "Grab")
		if err != nil {
			log.Printf("Could not create MediaWiki client: %s\n", err)
			c.String(http.StatusInternalServerError, "error logging into mediawiki: %s", err.Error())
			return
		}
		err = w.Login(instance.MediaWikiUname, instance.MediaWikiPword)
		if err != nil {
			log.Printf("Could not login to MediaWiki: %s\n", err)
			c.String(http.StatusInternalServerError, "error logging into mediawiki: %s", err.Error())
			return
		}

		fmt.Printf("Type is %s\n", payload.Type)

		if payload.Type == "shortcut" {
			fmt.Println(payload)
			fmt.Println(payload.Message.Timestamp)
			fmt.Println(payload.Channel.ID)
			fmt.Printf("%s,%s\n", payload.Container.ChannelID, payload.Container.ThreadTs)
			modalRequest := generateModalRequest(payload.Message.Channel, payload.Message.ThreadTimestamp)
			_, err = slackClient.OpenView(payload.TriggerID, modalRequest)
			if err != nil {
				fmt.Printf("Error opening view: %s", err)
			}
		} else if payload.Type == "view_submission" {
			articleTitle := payload.View.State.Values["Article Title"]["articleTitle"].Value
			sectionTitle := payload.View.State.Values["Section Title"]["sectionTitle"].Value
			clobberValue := payload.View.State.Values["Clobber"]["clobber"].SelectedOptions[0].Value
			clobber := false
			if clobberValue == "confirmed" {
				clobber = true
			}

			// Ack so we don't die when eating large messages
			log.Println("Command received. Saving thread...")
			c.String(http.StatusOK, "Command received. Saving thread...")

			fmt.Println(payload.Channel.ID)
			fmt.Println(payload.Message.ThreadTimestamp)

			// OK, now actually post it to the wiki.
			var conversation []slack.Message
			var transcript string
			conversation, err = getThreadConversation(slackClient, payload.Channel.ID, payload.Container.ThreadTs)
			if err != nil {
				log.Println("Failed to get thread conversation: ", err)
				c.String(http.StatusInternalServerError, "Failed to get thread conversation: %s", err.Error())
				return
			}

			if articleTitle == "" {
				// Get title if not provided
				articleTitle, transcript, err = generateTranscript(&instance, slackClient, w, conversation)
			} else {
				_, transcript, err = generateTranscript(&instance, slackClient, w, conversation)
			}

			if err != nil {
				log.Println("Error generating transcript: ", err)
				c.String(http.StatusInternalServerError, "Error generating transcript: %s", err.Error())
				return
			}

			log.Println("Thread downloaded. Publishing to wiki...")
			c.String(http.StatusOK, "Thread downloaded. Publishing to wiki...")

			// Publish the content to the wiki. If the article doesn't exist,
			// then create it. If the section doesn't exist, then create it.
			err = publishToWiki(w, clobber, articleTitle, sectionTitle, transcript)
			if err != nil {
				log.Println("Error publishing to wiki: ", err)
				c.String(http.StatusInternalServerError, "Error publishing to wiki: %s", err.Error())
				return
			}

			// Update the ephemeral message
			newArticleURL, _, err := getArticleURL(w, articleTitle)
			if err != nil {
				log.Println("Could not get article URL: ", err)
				c.String(http.StatusInternalServerError, "Error getting article URL: %s", err.Error())
				return
			}
			responseData := fmt.Sprintf(
				`{"replace_original": "true", "thread_ts": "%s", "text": "Article updated! You can find it posted at: %s"}`,
				payload.Message.ThreadTimestamp,
				newArticleURL,
			)
			reader := strings.NewReader(responseData)
			_, err = http.Post(payload.ResponseURL, "application/json", reader)

			if err != nil {
				log.Printf("Failed updating message: %v", err)
			}
		}
	}
}

func getThreadConversation(api *slack.Client, channelID string, threadTs string) (conversation []slack.Message, err error) {
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
func generateTranscript(instance *Instance, api *slack.Client, w *mwclient.Client, conversation []slack.Message) (title string, transcript string, err error) {
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

	// Remove messages sent by Grab	and mentioning Grab
	// Format conversation into string line-by-line
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
				log.Println(err)
			} else {
				conversationUsers[message.User] = msgUser.Name
			}
		}
		msgUserName := conversationUsers[message.User]

		transcript += msgUserName + ": " + message.Text + "\n\n"
		// fmt.Printf("[%s] %s: %s\n", message.Timestamp, message.User, message.Text)

		// Check for attachements
		for _, attachment := range message.Attachments {
			// Dead-simple way to grab text attachments.
			if attachment.Text != "" {
				transcript += "\n\n<pre>" + attachment.Text + "</pre>"
			}
		}

		// I guess files are different.
		for _, file := range message.Files {
			// Useful Debugging things
			//fmt.Println(file.Mimetype)
			//fmt.Println(file.URLPrivateDownload)

			// Download the file from Slack
			basename := fmt.Sprintf("%s.%s", uuid.New(), file.Filetype)
			path := fmt.Sprintf("/tmp/%s", basename)
			var tempFile *os.File
			tempFile, err = os.Create(path)
			defer os.Remove(path)
			if err != nil {
				log.Println("Error creating output file:", err)
				return
			}
			err = api.GetFile(file.URLPrivateDownload, tempFile)
			if err != nil {
				log.Println("Error getting file from Slack: ", err)
				return
			}
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
				fileTitle, err = uploadToWiki(instance, w, path)
				if err != nil {
					log.Println("Error uploading file: ", err)
					return
				}
				// It'll be like uhhh [[File:name.jpg]] or whatever.
				transcript += fmt.Sprintf("[[File:%s]]\n\n", fileTitle)
			} else if strings.Contains(file.Mimetype, "text") {
				var fileContents []byte
				fileContents, err = os.ReadFile(path)
				if err != nil {
					log.Println("Error reading file: ", err)
					return
				}
				transcript += file.Name + ":\n<pre>" + string(fileContents) + "</pre>\n\n"
			}
		}
	}

	return pureConversation[0].Text, transcript, nil
}
