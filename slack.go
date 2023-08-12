package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/antonholmquist/jason"
	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/google/uuid"
)

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
		instance.MediaWikiDomain = c.Query("mediaWikiDomain")

		err = insertInstance(db, instance)
		if err != nil {
			c.String(http.StatusInternalServerError, "error storing slack access token: %s", err.Error())
			return
		}

		/*
			if err = db.Update(func(tx *bolt.Tx) error {
				bucket := tx.Bucket([]byte("tokens"))
				if bucket == nil {
					return errors.New("error accessing tokens bucket")
				}
				return bucket.Put([]byte(resp.Team.ID), []byte(resp.AccessToken))
			}); err != nil {
				c.String(http.StatusInternalServerError, "error storing slack access token: %s", err.Error())
				return
			}
		*/
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
				handleMention(am)
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

// DEBUG: Fuck fuck fuck
func handleMention(am *slackevents.AppMentionEvent) (err error) {

	//command, err := interpretCommand(tokenizeCommand(ev.Text))
	return nil
}

/*
// Code to run if someone mentions the bot.
func handleMention(ev *slackevents.EventsAPIInnerEvent) {
	command, err := interpretCommand(tokenizeCommand(ev.Text))
	if err != nil {
		log.Println(err)
		return
	}

	var conversation []slack.Message

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

		conversation, err = getThreadConversation(ev.Channel, ev.ThreadTimeStamp)
		if err != nil {
			log.Println(err)
			return
		}
	} else if command.rangeHappened {
		oldestTs, latestTs := formatRange(*command.rangeOpts.oldest, *command.rangeOpts.latest)

		// Get the conversation history
		conversation, err = getConversation(ev.Channel, oldestTs, latestTs)
		if err != nil {
			fmt.Printf("Could not get messages: %v", err)
		}

		// Reverse it so it's in chronological order
		for i, j := 0, len(conversation)-1; i < j; i, j = i+1, j-1 {
			conversation[i], conversation[j] = conversation[j], conversation[i]
		}
	} else {
		// Post ephemeral message to user
		_, err = client.PostEphemeral(ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText(helpMessage, false))
		if err != nil {
			fmt.Printf("failed posting message: %v", err)
		}
		return
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
		return
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
		return
	}

	// Now that it has been published and definitely exists, get
	// the URL again
	if missing {
		newArticleURL, _, err = getArticleURL(*command.title)
		if err != nil {
			fmt.Println(err)
			return
		}
	}

	// Post ephemeral message to user
	_, err = client.PostEphemeral(ev.Channel, ev.User, slack.MsgOptionTS(ev.ThreadTimeStamp), slack.MsgOptionText(fmt.Sprintf("Article saved! You can find it at: %s", newArticleURL), false))
	if err != nil {
		fmt.Printf("failed posting message: %v", err)
	}

}

*/

type GrabCallbackIDs string
type GrabBlockActionIDs string

const (
	// Callback ID
	GrabInteractionAppendThreadTranscript = "append_thread_transcript"
	// Block Action IDs for that Callback ID
	GrabInteractionAppendThreadTranscriptConfirm = "append_thread_transcript_confirm"
	GrabInteractionAppendThreadTranscriptCancel  = "append_thread_transcript_cancel"
)

func interactionResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		var payload slack.InteractionCallback
		err := json.Unmarshal([]byte(c.Request.FormValue("payload")), &payload)
		if err != nil {
			c.String(http.StatusInternalServerError, "error reading slack interaction payload: %s", err.Error())
			return
		}
		//fmt.Println("THIS IS YOUR PAYLOAD >>> ", payload)
		//fmt.Println(payload.ActionCallback.BlockActions[0])
		//fmt.Println(payload.Type)

		if payload.Type == "message_action" {
			if payload.CallbackID == GrabInteractionAppendThreadTranscript {
				// Define blocks

				messageText := slack.NewSectionBlock(
					slack.NewTextBlockObject(
						"mrkdwn",
						"Saving thread transcript! Please provide some article info. You can specify existing articles and sections, or come up with new ones.",
						false,
						false,
					),
					nil,
					nil,
				)
				articleTitleText := slack.NewTextBlockObject("plain_text", "Article Title", false, false)
				articleTitlePlaceholder := slack.NewTextBlockObject("plain_text", "Provide a title for this article", false, false)
				articleTitleElement := slack.NewPlainTextInputBlockElement(articleTitlePlaceholder, "article_title")
				// Notice that blockID is a unique identifier for a block
				articleTitle := slack.NewInputBlock("Article Title", articleTitleText, nil, articleTitleElement)

				articleSectionText := slack.NewTextBlockObject("plain_text", "Article Section", false, false)
				articleSectionPlaceholder := slack.NewTextBlockObject("plain_text", "Optionally, place it under a section", false, false)
				articleSectionElement := slack.NewPlainTextInputBlockElement(articleSectionPlaceholder, "article_section")
				// Notice that blockID is a unique identifier for a block
				articleSection := slack.NewInputBlock("Article Section", articleSectionText, nil, articleSectionElement)

				confirmButton := slack.NewButtonBlockElement(
					GrabInteractionAppendThreadTranscriptConfirm,
					"CONFIRM",
					slack.NewTextBlockObject("plain_text", "CONFIRM", false, false),
				)
				confirmButton.Style = "primary"

				cancelButton := slack.NewButtonBlockElement(
					GrabInteractionAppendThreadTranscriptCancel,
					"CANCEL",
					slack.NewTextBlockObject("plain_text", "CANCEL", false, false),
				)

				buttons := slack.NewActionBlock(
					"",
					confirmButton,
					cancelButton,
				)

				blockMsg := slack.MsgOptionBlocks(
					messageText,
					articleTitle,
					articleSection,
					buttons,
				)
				var instance Instance
				instance, err = selectInstanceByTeamID(db, payload.User.TeamID)
				if err != nil {
					log.Println(err)
					c.String(http.StatusInternalServerError, "error reading slack access token: %s", err.Error())
				}

				_, err = slack.New(instance.SlackAccessToken).PostEphemeral(
					payload.Channel.ID,
					payload.User.ID,
					slack.MsgOptionTS(payload.Message.ThreadTimestamp),
					blockMsg,
				)
				if err != nil {
					fmt.Println(err)
					c.String(http.StatusInternalServerError, "error posting ephemeral message: %s", err.Error())
					return
				}
			}
		} else if payload.Type == "block_actions" {
			firstBlockAction := payload.ActionCallback.BlockActions[0]
			if firstBlockAction.ActionID == GrabInteractionAppendThreadTranscriptConfirm {
				v, err := jason.NewObjectFromBytes(payload.RawState)
				if err != nil {
					log.Println(err)
					c.String(http.StatusInternalServerError, "error saving to wiki: %s", err.Error())
					return
				}
				articleTitle, err := v.GetString("values", "Article Title", "article_title", "value")
				if err != nil {
					log.Println(err)
					c.String(http.StatusInternalServerError, "error saving to wiki: %s", err.Error())
					return
				}
				articleSection, err := v.GetString("values", "Article Section", "article_section", "value")
				if err != nil {
					log.Println(err)
					c.String(http.StatusInternalServerError, "error saving to wiki: %s", err.Error())
					return
				}

				fmt.Println(articleTitle, " / ", articleSection)

				// OK, now actually post it to the wiki.
				/*
					conversation, err = getThreadConversation(channelID, threadTs)
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
						threadTs,
						newArticleURL,
					)
					reader := strings.NewReader(responseData)
					_, err = http.Post(callback.ResponseURL, "application/json", reader)

					if err != nil {
						log.Printf("Failed updating message: %v", err)
					}

				*/
			} else if firstBlockAction.ActionID == GrabInteractionAppendThreadTranscriptCancel {
				// Update the ephemeral message
				responseData := fmt.Sprintf(
					`{"replace_original": "true", "thread_ts": "%s", "text": "Grab request cancelled."}`,
					payload.Container.ThreadTs,
				)
				reader := strings.NewReader(responseData)
				_, err := http.Post(payload.ResponseURL, "application/json", reader)

				if err != nil {
					log.Printf("Failed updating message: %v", err)
				}
			}
		}
		return
	}
}

/*
func appendResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		return
	}
}

func rangeResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		return
	}
}

func interactResp() func(c *gin.Context) {
	return func(c *gin.Context) {
		return
	}
}
*/
