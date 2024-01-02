package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
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
	AppendThread = "append_thread_msg"
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

func interactionResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		// Get raw HTTP request into useful Slack object
		var payload slack.InteractionCallback
		err := json.Unmarshal([]byte(c.Request.FormValue("payload")), &payload)
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading slack interaction payload: %s", err.Error())
			return
		}

		// If it's not a modal action, we don't care.
		validPayloads := []string{"view_submission", "message_action"}
		if slices.Contains(validPayloads, string(payload.Type)) == false {
			log.Println("Invalid payload type: ", payload.Type)
			c.String(http.StatusBadRequest, "Invalid payload type: %s", payload.Type)
			return
		}

		// Pull credentials out of DB
		var instance Instance
		instance, err = selectInstanceByTeamID(db, payload.User.TeamID)
		if err != nil {
			log.Println("Could not get credentials from DB", err)
			c.String(http.StatusInternalServerError, "error reading slack access token: %s", err.Error())
		}

		s := NewSlackBridge(instance)

		switch payload.Type {
		case "message_action":
			err := s.handleMessageAction(payload)
			if err != nil {
				fmt.Printf("Error handling message_action: %s", err)
				c.String(http.StatusInternalServerError, "Error handling message_action: %s", err.Error())
			}
		case "view_submission":
			err := s.handleViewSubmission(c, payload, instance)
			if err != nil {
				fmt.Printf("Error handling view_submission: %s", err)
				c.String(http.StatusInternalServerError, "Error handling view_submission: %s", err.Error())
			}
		}
	}
}

func (s *SlackBridge) handleMessageAction(payload slack.InteractionCallback) (err error) {
	modalRequest := s.generateModalRequest(payload.Channel.ID, payload.Message.ThreadTimestamp, payload.User.ID)
	_, err = s.api.OpenView(payload.TriggerID, modalRequest)
	if err != nil {
		return err
	}

	return nil
}

func (s *SlackBridge) handleViewSubmission(c *gin.Context, payload slack.InteractionCallback, instance Instance) (err error) {
	articleTitle := payload.View.State.Values["Article Title"]["articleTitle"].Value
	sectionTitle := payload.View.State.Values["Section Title"]["sectionTitle"].Value
	clobberValue := payload.View.State.Values["Clobber"]["clobber"].SelectedOptions[0].Value
	clobber := false
	if clobberValue == "confirmed" {
		clobber = true
	}

	// Get the Thread into a common form
	fmt.Println("Channel ID and Thread TS: ", payload.Channel.ID, payload.Message.ThreadTimestamp)
	messageContext := strings.Split(payload.View.ExternalID, ",")
	channelID := messageContext[0]
	threadTS := messageContext[1]
	userID := messageContext[2]

	thread, err := s.getThread(channelID, threadTS)
	if err != nil {
		return err
	}

	// If we didn't get a title, then grab and truncate the first message
	if len(articleTitle) == 0 {
		articleTitle = thread.getTitle()
	}

	// Ack so we don't die when eating large messages
	c.String(http.StatusOK, "")

	// Figure out what kind of Wiki this org has
	var w WikiBridge
	if len(instance.MediaWikiURL) > 0 {
		wiki, err := NewMediaWikiBridge(instance)
		w = &wiki // Forgive me father for I have sinned
		if err != nil {
			return err
		}
	}

	// Post Thread to Wiki
	transcript := w.generateTranscript(thread)
	url, err := w.uploadArticle(articleTitle, sectionTitle, transcript, clobber)

	// Let the user know where the page is
	responseData := fmt.Sprintf(
		`Article saved! You can find it posted at: %s`,
		url,
	)

	_, err = s.api.PostEphemeral(
		channelID,
		userID,
		slack.MsgOptionTS(threadTS),
		slack.MsgOptionText(responseData, false),
	)
	if err != nil {
		return err
	}

	return nil
}
