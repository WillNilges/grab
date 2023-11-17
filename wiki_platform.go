package main

// Generic types for pages on a customer's wiki. Wiki software is almost
// certainly too generic for this to be useful, but I guess I can try.
type WikiPage struct {
	title string
	content []WikiSection
	url string
}

// A generic section on someone's wiki. This might be more useful than
// the above, actually, since I can dump transcripts into this.
type WikiSection struct {
	title string
	content string
}

// A WikiPlatform is Grab's generic interface for any kind of knowledge base.
// (Such as MediaWiki, BookStack, Confluence, etc)
// There's not that much that it needs to be able to do generically.
type WikiPlatform interface {
	publish() error
	upload() error
	fetch() (string, error)
}
