package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/google/uuid"
)

type GrabCallbackIDs string
type GrabBlockActionIDs string

const (
	// Callback ID
	AppendThread = "append_thread_transcript"
	// Block Action IDs for that Callback ID
	AppendThreadConfirm = "append_thread_transcript_confirm"
	AppendThreadCancel  = "append_thread_transcript_cancel"
)

// Middleware to verify integrity of API calls from Slack
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

// Register a user with Grab. Get Slack credentials and Wiki credentials
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

// Respond to Event
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
				httpStatus, err := handleMention(ce, am)
				if err != nil {
					log.Println("Error responding to AppMention Event: ", err)
					c.String(httpStatus, "Error responding to AppMention Event: %s", err.Error())
				}
				c.String(http.StatusOK, "ack")
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

// TODO: Somehow track how many requests were made per minute or whatever, don't
// duplicate requests in the same thread
func handleMention(ce *slackevents.EventsAPICallbackEvent, am *slackevents.AppMentionEvent) (httpStatus int, err error) {
	// Retrieve credentials to log into Slack and MediaWiki
	var instance Instance
	instance, err = selectInstanceByTeamID(db, ce.TeamID)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	s := NewSlackBridge(instance)

	// First of all, are we in a thread?
	if am.ThreadTimeStamp == "" {
		_, err = s.api.PostEphemeral(
			am.Channel,
			am.User,
			slack.MsgOptionTS(am.ThreadTimeStamp),
			slack.MsgOptionText(
				"Sorry, I only work inside threads!",
				false,
			),
		)
		if err != nil {
			log.Printf("failed posting message: %v\n", err)
			return http.StatusInternalServerError, err
		}
		return http.StatusBadRequest, errors.New("This function only works inside of threads")
	}

	/*
		// Clean up old Grab messages
		// FIXME: This doesn't work, probably due to ephemeral messages.
		// This brings up a problem with having to ping Grab. Garbage will build
		// up in the thread (@Grab's and messages from Grab for the user)
		s := NewSlackBridge(instance)
		conversation, err := s.getConversationReplies(am.Channel, am.ThreadTimeStamp)
		if err != nil {
			log.Println("Could not get conversation: ", err)
		}

		// Get the bot's userID
		authTestResponse, err := s.api.AuthTest()
		if err != nil {
			log.Fatalf("Error calling AuthTest: %s", err)
		}

		for _, message := range conversation {
			if message.User == authTestResponse.UserID || strings.Contains(message.Text, fmt.Sprintf("<@%s>", authTestResponse.UserID)) {
				continue
			}

			_, _, err := s.api.DeleteMessage(am.Channel, message.Timestamp)
			if err != nil {
				log.Println("Could not delete message from Grab: ", err)
			}
		}
		// </Clean up old Grab messages>
	*/

	blockMsg := s.createBlockMessage()

	_, err = s.api.PostEphemeral(
		am.Channel,
		am.User,
		slack.MsgOptionTS(am.ThreadTimeStamp),
		blockMsg,
	)
	if err != nil {
		log.Println("Error posting ephemeral message: ", err)
		return http.StatusInternalServerError, err
	}
	return http.StatusOK, nil
}

func interactionResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		// Get raw HTTP request into useful Slack object
		var payload slack.InteractionCallback
		err := json.Unmarshal([]byte(c.Request.FormValue("payload")), &payload)
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading slack interaction payload: %s", err.Error())
			return
		}

		// If it's not a block action, we don't care.
		if payload.Type != "block_actions" {
			c.String(http.StatusBadRequest, "Invalid payload type: %s", payload.Type)
			return
		}

		// Pull credentials out of DB
		var instance Instance
		instance, err = selectInstanceByTeamID(db, payload.User.TeamID)
		if err != nil {
			log.Println(err)
			c.String(http.StatusInternalServerError, "error reading slack access token: %s", err.Error())
		}

		// It must be one of two things:
		// User went through with a Grab
		// User cancelled a Grab
		firstBlockAction := payload.ActionCallback.BlockActions[0]
		if firstBlockAction.ActionID == AppendThreadConfirm {
			s := NewSlackBridge(instance)

			// Parse form values
			articleTitle, articleSection, clobber, err := s.parseSlackForm(payload.RawState)
			if err != nil {
				c.String(http.StatusInternalServerError, "Could not parse form: %s", err)
				return
			}

			// Ack so we don't die when eating large messages
			log.Println("Command received. Saving thread...")
			c.String(http.StatusOK, "Command received. Saving thread...")

			// Get the Thread into a common form
			thread, err := s.getThread(payload.Channel.ID, payload.Container.ThreadTs)

			// Figure out what kind of Wiki this org has
			var w WikiBridge
			if len(instance.MediaWikiURL) > 0 {
				wiki, err := NewMediaWikiBridge(instance)
				w = &wiki // Forgive me father for I have sinned
				if err != nil {
					log.Printf("error logging into mediawiki: %s\n", err.Error())
					c.String(http.StatusInternalServerError, "error logging into mediawiki: %s", err.Error())
					return
				}
			}

			// If we didn't get a title, then grab and truncate the first message
			if len(articleTitle) == 0 {
				articleTitle = thread.getTitle()
			}

			// Post Thread to Wiki
			transcript := w.generateTranscript(thread)
			url, err := w.uploadArticle(articleTitle, articleSection, transcript, clobber)

			// Update the ephemeral message
			responseData := fmt.Sprintf(
				`{"replace_original": "true", "thread_ts": "%s", "text": "Article saved! You can find it posted at: %s"}`,
				payload.Container.ThreadTs,
				url,
			)
			reader := strings.NewReader(responseData)
			_, err = http.Post(payload.ResponseURL, "application/json", reader)

			if err != nil {
				log.Printf("Failed updating message: %v", err)
				c.String(http.StatusInternalServerError, "Failed updating message: %s", err.Error())
				return
			}
		} else if firstBlockAction.ActionID == AppendThreadCancel {
			// Update the ephemeral message
			responseData := fmt.Sprintf(
				`{"replace_original": "true", "thread_ts": "%s", "text": "Grab request cancelled."}`,
				payload.Container.ThreadTs,
			)
			reader := strings.NewReader(responseData)
			_, err := http.Post(payload.ResponseURL, "application/json", reader)

			if err != nil {
				log.Printf("Failed updating message: %v", err)
				c.String(http.StatusInternalServerError, "Failed updating message: %s", err.Error())
				return
			}
		}

	}
}
