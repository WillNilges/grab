package main

type ChatServicer interface {
	getThread(...) Thread
}

type WikiServicer interface {
	generateTranscript() (title string, transcript string)
	uploadArticle()
	uploadImage()
}
