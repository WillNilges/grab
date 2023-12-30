package main

import "time"

type Thread struct {
	Title string
	Timestamp time.Time
	Messages []Message
}

type Message struct {
	Timestamp time.Time
	Author string
	Text string
	Files []string // URL to file associated with this (hopefully only pictures)
}
