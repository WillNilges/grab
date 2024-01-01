package main

import "time"

type Thread struct {
	Timestamp time.Time
	Messages  []Message
}

func (t *Thread) getTitle() string {
	var title string
	title = t.Messages[0].Text

	// Truncate the first message to 32 characters
	if len(title) > 32 {
		title = title[0:32]
	}
	return title
}

func (t *Thread) getNames() []string {
	// Create a map to store unique names
	uniqueNames := make(map[string]struct{})

	// Iterate over the array and add names to the map
	for _, message := range t.Messages {
		uniqueNames[message.Author] = struct{}{}
	}

	// Extract unique names from the map
	var uniqueNamesSlice []string
	for name := range uniqueNames {
		uniqueNamesSlice = append(uniqueNamesSlice, name)
	}
	return uniqueNamesSlice
}

type Message struct {
	Timestamp time.Time
	Author    string
	Text      string
	Files     []string // URL to file associated with this (hopefully only pictures)
}
