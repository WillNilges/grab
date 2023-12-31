package main

type ChatBridge interface {
	getThread() Thread
	saveImage(url string)
}

type WikiBridge interface {
	generateTranscript(thread Thread) (transcript string)
	uploadArticle(title string, section string, transcript string, clobber bool) (url string, err error)
	uploadImage(path string) (filename string, err error)
}
