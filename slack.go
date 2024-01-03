package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"

	"github.com/google/uuid"
)

type SlackBridge struct {
	api *slack.Client
}

func NewSlackBridge(instance Instance) (s SlackBridge) {
	s.api = slack.New(instance.SlackAccessToken)
	return s
}

func (s *SlackBridge) getThread(channelID string, threadTs string) (thread Thread, err error) {
	conversation, err := s.getConversationReplies(channelID, threadTs)
	if err != nil {
		return Thread{}, err
	}

	// Get the bot's userID
	authTestResponse, err := s.api.AuthTest()
	if err != nil {
		log.Fatalf("Error calling AuthTest: %s", err)
	}

	// The ThreadTS is when this party started
	thread.Timestamp = s.slackTSToTime(threadTs)

	conversationUsers := map[string]string{}
	for _, message := range conversation {
		// Don't include messages from Grab or that mention Grab.
		if message.User == authTestResponse.UserID || strings.Contains(message.Text, fmt.Sprintf("<@%s>", authTestResponse.UserID)) {
			continue
		}

		// Build a Message. Convert Slack Message into our format
		m := Message{}

		// Translate the user id to a user name. Cache them so we don't have
		// to hit the API every time
		var msgUser *slack.User
		if len(conversationUsers[message.User]) == 0 {
			msgUser, err = s.api.GetUserInfo(message.User)
			if err != nil {
				log.Println(err)
			} else {
				conversationUsers[message.User] = msgUser.Name
			}
		}

		m.Timestamp = s.slackTSToTime(message.Timestamp)
		m.Author = conversationUsers[message.User]
		m.Text = s.mrkdwnToMarkdown(message.Text)

		// Check for attachements
		for _, attachment := range message.Attachments {
			// Dead-simple way to grab text attachments. I guess Grab Messages
			// will be Markdown.
			if attachment.Text != "" {
				m.Text += "\n\n```" + attachment.Text + "```"
			}
		}

		// Check for files
		for _, file := range message.Files {
			path, err := s.getFile(file)
			if err != nil {
				log.Println("Could not save file: ", err)
			}
			m.Files = append(m.Files, path)
		}

		thread.Messages = append(thread.Messages, m)
	}

	return thread, nil
}

// Interaction Handlers

func (s *SlackBridge) handleMessageAction(payload slack.InteractionCallback) (err error) {
	modalRequest := s.generateTitleFormRequest(payload.Channel.ID, payload.Message.ThreadTimestamp, payload.User.ID)
	_, err = s.api.OpenView(payload.TriggerID, modalRequest)
	if err != nil {
		return err
	}

	return nil
}

func (s *SlackBridge) handleViewSubmission(c *gin.Context, payload slack.InteractionCallback, instance Instance) (err error) {
	articleTitle := payload.View.State.Values["Article Title"]["articleTitle"].Value
	sectionTitle := payload.View.State.Values["Section Title"]["sectionTitle"].Value
	var clobber bool
	if len(payload.View.State.Values["Clobber"]["clobber"].SelectedOptions) == 0 {
		clobber = false
	} else {
		clobberValue := payload.View.State.Values["Clobber"]["clobber"].SelectedOptions[0].Value
		if clobberValue == "confirmed" {
			clobber = true
		}
	}

	// Get the Thread into a common form
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

	// If all that worked, ACK so we don't die when eating large messages
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
	responseData := fmt.Sprintf("Article saved! You can find it at: %s", url)

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

// Utility Functions

func (s *SlackBridge) getConversationReplies(channelID string, threadTs string) (conversation []slack.Message, err error) {
	// Get the conversation history
	params := slack.GetConversationRepliesParameters{
		ChannelID: channelID,
		Timestamp: threadTs,
	}
	conversation, _, _, err = s.api.GetConversationReplies(&params)
	if err != nil {
		return conversation, err
	}
	return conversation, nil
}

func (s *SlackBridge) getFile(file slack.File) (path string, err error) {
	basename := fmt.Sprintf("%s.%s", uuid.New(), file.Filetype)
	path = fmt.Sprintf("/tmp/grab/%s", basename)
	var tempFile *os.File
	tempFile, err = os.Create(path)
	if err != nil {
		log.Println("Error creating output file:", err)
		return
	}
	err = s.api.GetFile(file.URLPrivateDownload, tempFile)
	if err != nil {
		log.Println("Error getting file from Slack: ", err)
		return
	}
	tempFile.Close()
	return path, nil
}

func (s *SlackBridge) slackTSToTime(slackTimestamp string) (slackTime time.Time) {
	// Convert the Slack timestamp to a Unix timestamp (float64)
	slackUnixTimestamp, err := strconv.ParseFloat(strings.Split(slackTimestamp, ".")[0], 64)
	if err != nil {
		fmt.Println("Error parsing Slack timestamp:", err)
		return
	}

	// Create a time.Time object from the Unix timestamp (assuming UTC time zone)
	slackTime = time.Unix(int64(slackUnixTimestamp), 0)
	return slackTime
}

func (s *SlackBridge) generateTitleFormRequest(channelID string, threadTS string, user string) slack.ModalViewRequest {
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
	// Ooops all optional

	articleTitle.Optional = true // People shouldn't write go
	sectionTitle.Optional = true // People shouldn't write go
	clobberBox.Optional = true   // People shouldn't write go

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
	modalRequest.ExternalID = fmt.Sprintf("%s,%s,%s", channelID, threadTS, user)
	return modalRequest
}

// REALLY SHITTY parser from ChatGPT. I spent some time fucking around with the 
// Blocks and have concluded that writing a parser for that shit is a whole other
// project in and of itself. Maybe someday. For now, my shit will probably be
// vulnerable to regex-based attacks.
func (s *SlackBridge) mrkdwnToMarkdown(input string) string {
	// Handle bold text
	boldRegex := regexp.MustCompile(`\*(.*?)\*`)
	input = boldRegex.ReplaceAllString(input, "**$1**")

	// Handle italic text
	italicRegex := regexp.MustCompile(`_(.*?)_`)
	input = italicRegex.ReplaceAllString(input, "*$1*")

	// Handle strikethrough text
	strikeRegex := regexp.MustCompile(`~(.*?)~`)
	input = strikeRegex.ReplaceAllString(input, "~~$1~~")

	// Handle code blocks
	codeRegex := regexp.MustCompile("`([^`]+)`")
	input = codeRegex.ReplaceAllString(input, "`$1`")

	// Handle links with labels
	linkWithLabelRegex := regexp.MustCompile(`<([^|]+)\|([^>]+)>`)
	input = linkWithLabelRegex.ReplaceAllString(input, "[$2]($1)")

	// Handle links without labels
	linkWithoutLabelRegex := regexp.MustCompile(`<([^>]+)>`)
	input = linkWithoutLabelRegex.ReplaceAllString(input, "[$1]($1)")

	return input
}
