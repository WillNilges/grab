package main

// A message is Grab's way to interpret something that somebody
// has said. It will be a unifying agent between all platforms that
// Grab supports.
type Message struct {
	name string
	handle string
	timestamp string
	attachment []Attachment
}

// Grab doesn't store any user data, but it will be helpful to
// treat anything that isn't text inside a user's message as an
// Attachment that links to wherever it's stored.
type Attachment struct {
	title string
	sourceURL string
	destURL string
}

// A ChatPlatform is a generic way for Grab to treat any and all
// apps it is connected with on the input side. This includes
// Slack, Discord, etc.
type ChatPlatform interface {
	getConversation() []Message
}
