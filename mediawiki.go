package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/EricMCarroll/go-mwclient"
	"github.com/antonholmquist/jason"
)

type MediaWikiBridge struct {
	api *mwclient.Client
}

func NewMediaWikiBridge(instance Instance) (wiki MediaWikiBridge, err error) {
	w, err := mwclient.New(instance.MediaWikiURL, "Grab")
	if err != nil {
		return MediaWikiBridge{}, err
	}
	err = w.Login(instance.MediaWikiUname, instance.MediaWikiPword)
	if err != nil {
		return MediaWikiBridge{}, err
	}

	wiki.api = w
	return wiki, nil
}

// TODO: Convert markdown(?) to MediaWiki markup
func (w *MediaWikiBridge) generateTranscript(thread Thread) (transcript string) {
	// Define the desired format layout
	timeLayout := "2006-01-02 at 15:04"
	transcriptBegin := thread.Timestamp.Format(timeLayout)
	currentTime := time.Now().Format(timeLayout)

	transcript += "Transcript generated at " + currentTime + ".\n\n"
	transcript += "Conversation begins at " + transcriptBegin + ".\n\n"


	for _, m := range thread.Messages {
		transcript += m.Author + ": " + m.Text + "\n\n"

		// Files will be handled in the wiki. We will download them over in the
		// chat bridge and then we will, on each message, have the path and title
		// so that we can call them up and upload them in context here.
		// TODO: Upload and link(?) files to messages (so they're in context)
	}

	return transcript
}


// Helper function for putting things on the wiki. Can easily control how content
// gets published by setting or removing variables
//func publishToWiki(w *mwclient.Client, clobber bool, title string, sectionTitle string, convo string) (err error) {
func (w *MediaWikiBridge) uploadArticle(title string, section string, transcript string, clobber bool) (url string, err error) {
	parameters := map[string]string{
		"action":     "edit",
		"title":      title,
		"appendtext": "\n\n" + transcript, // Append some newlines so he gets formatted nicely
		"bot":        "true",
		"summary":    "Grab uploadArticle append section " + section,
	}

	if clobber {
		delete(parameters, "appendtext")
		parameters["text"] = transcript
		parameters["summary"] = fmt.Sprintf("Grab uploadArticle clobber section %s", section)
	}

	if section != "" {
		sectionExists, _ := w.sectionExists(title, section)
		if sectionExists && clobber {
			// If we're clobbering a section, we need to delete it and then
			// re-make it. Absolutely not ideal, because if a section exists
			// in the middle of the page, it will move it to the end.
			index, err := w.findSectionId(title, section)
			if err != nil {
				return "", err
			}
			parameters["section"] = index
			parameters["text"] = ""
			w.api.Edit(parameters) // Delete the section

			parameters["section"] = "new"
			parameters["sectiontitle"] = section
			parameters["text"] = transcript 
		} else if sectionExists /* && append */ {
			index, err := w.findSectionId(title, section)
			if err != nil {
				return "", err
			}
			parameters["section"] = index
		} else {
			parameters["section"] = "new"
			parameters["sectiontitle"] = section
		}
	}

	// Make the request.
	err = w.api.Edit(parameters)
	if err != nil {
		log.Println("Failed to make edit: ", err)
		return "", err
	}

	// Get and return the URL
	url, _, err = w.getArticleURL(title)
	if err != nil {
		log.Println("Could not get article URL: ", err)
		return "", err
	}

	return url, nil
}

// THIS IS FOR FILES AND NEEDS TO CHANGE
func uploadToWiki(instance *Instance, w *mwclient.Client, path string) (filename string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}

	// Set up an HTTP client
	// TODO: Can we just hijack the mwclient's http client?
	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{
		Timeout: time.Second * 10,
		Jar:     cookieJar,
	}

	// --- Authentication ---
	// Steal cookies from the mediawiki library
	cookieURL, _ := url.Parse(instance.MediaWikiURL)
	cookieJar.SetCookies(cookieURL, w.DumpCookies())

	// Get csrf token
	csrfToken, err := w.GetToken(mwclient.CSRFToken)
	if err != nil {
		return "", err
	}

	// New multipart writer.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Get the file's name
	fileInfo, err := file.Stat()
	if err != nil {
		log.Println("Error:", err)
		return "", err
	}
	// Extract the basename from the file's name
	basename := filepath.Base(fileInfo.Name())
	//log.Println("File size: ", fileInfo.Size())

	// Parameters for file
	writer.WriteField("action", "upload")
	writer.WriteField("format", "json")
	writer.WriteField("filename", basename)
	writer.WriteField("comment", "Attachment from Slack.")
	writer.WriteField("token", csrfToken)

	fw, err := writer.CreateFormFile("file", basename)
	if err != nil {
		return basename, err
	}

	_, err = io.Copy(fw, file)
	if err != nil {
		return basename, err
	}
	writer.Close()
	req, err := http.NewRequest(http.MethodPost, instance.MediaWikiURL, bytes.NewReader(body.Bytes()))

	if err != nil {
		return basename, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rsp, err := client.Do(req)
	if err != nil {
		log.Println("An error occurred uploading content to wiki: ", err)
		return "", err
	}
	if rsp.StatusCode != http.StatusOK {
		log.Printf("Request failed with response code: %d", rsp.StatusCode)
	}

	responseBody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return basename, err
	}

	// MediaWiki will get angery if we try to upload a duplicate file. It will
	// kindly give us the name of the duplicate file, and we can just return that
	// and automagically, clobbering works properly again.
	if strings.Contains(string(responseBody), "duplicate") {
		rspJson, _ := jason.NewObjectFromBytes(responseBody)
		// Get the "duplicate" value
		warnings, err := rspJson.GetObject("upload", "warnings")
		if err != nil {
			log.Println("Jason Error:", err)
			return basename, err
		}

		duplicateArray, err := warnings.GetStringArray("duplicate")
		if err != nil {
			log.Println("Jason Error:", err)
			return basename, err
		}
		if len(duplicateArray) > 0 {
			log.Printf(`Warning: Found duplicate image "%s". Will use that one instead.`, duplicateArray[0])
			return duplicateArray[0], nil
		}
	}

	return basename, nil
}

func (w *MediaWikiBridge) getArticleURL(title string) (url string, missing bool, err error) {
	newArticleParameters := map[string]string{
		"action": "query",
		"format": "json",
		"titles": title,
		"prop":   "info",
		"inprop": "url",
	}

	newArticle, err := w.api.Get(newArticleParameters)
	if err != nil {
		return "", false, err
	}

	pages, err := newArticle.GetObjectArray("query", "pages")
	for _, page := range pages {
		url, err = page.GetString("canonicalurl")
		missing, _ = page.GetBoolean("missing")
		break // Just get first one. There won't ever not be just one.
	}

	if err != nil {
		return "", false, err
	}

	return url, missing, nil
}

// Check if the section exists or not, that's really all we care about (for now).
func (w *MediaWikiBridge) sectionExists(title string, section string) (exists bool, err error) {
	sectionQueryParameters := map[string]string{
		"format": "json",
		"action": "parse",
		"page":   title,
		"prop":   "sections",
	}

	sectionQuery, err := w.api.Get(sectionQueryParameters)
	if err != nil {
		return false, err
	}

	sections, err := sectionQuery.GetObjectArray("parse", "sections")
	for _, sect := range sections {
		var line string
		line, err = sect.GetString("line")
		if err != nil {
			return false, err
		}
		if line == section {
			return true, nil
		}
	}

	return false, nil
}

// Check if the section exists or not, that's really all we care about (for now).
func (w *MediaWikiBridge) findSectionId(title string, section string) (id string, err error) {
	sectionQueryParameters := map[string]string{
		"format": "json",
		"action": "parse",
		"page":   title,
		"prop":   "sections",
	}

	sectionQuery, err := w.api.Get(sectionQueryParameters)
	if err != nil {
		return "", err
	}

	sections, err := sectionQuery.GetObjectArray("parse", "sections")
	for _, sect := range sections {
		var line string
		line, err = sect.GetString("line")
		if err != nil {
			return "", err
		}
		if line == section {
			id, err = sect.GetString("index")
			if err != nil {
				return "", err
			}
			return id, nil
		}
	}

	return "", nil
}
