package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

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
		return
	}
}

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
