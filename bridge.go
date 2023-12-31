package main

type ChatBridge interface {
	getThread() Thread
	saveImage(url string)
}

type WikiBridge interface {
	generateTranscript(thread Thread) (transcript string)
	uploadArticle(title string, section string, transcript string, clobber bool) (url string, err error)
	//uploadImage(image string) // FIXME: I need a cache of some kind for this bullshit
}
