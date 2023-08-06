package main

import (
	"fmt"
	"os"
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"github.com/EricMCarroll/go-mwclient"
)

// Helper function for putting things on the wiki. Can easily control how content
// gets published by setting or removing variables
func publishToWiki(append bool, title string, sectionTitle string, convo string) (err error) {
	// Push conversation to the wiki, (overwriting whatever was already there, if Grab was the only person to edit?)
	parameters := map[string]string{
		"action":  "edit",
		"title":   title,
		"text":    convo,
		"bot":     "true",
		"summary": "Grab publishToWiki",
	}

	if sectionTitle != "" {
		sectionExists, _ := sectionExists(title, sectionTitle)
		if sectionExists && !append {
			// If we're clobbering a section, we need to delete it and then
			// re-make it. Absolutely not ideal, because if a section exists
			// in the middle of the page, it will move it to the end.
			index, err := findSectionId(title, sectionTitle)
			if err != nil {
				return err
			}
			parameters["section"] = index
			parameters["text"] = ""
			w.Edit(parameters) // Delete the section

			parameters["section"] = "new"
			parameters["sectiontitle"] = sectionTitle
			parameters["text"] = convo
			return w.Edit(parameters) // Make a new one

		} else if sectionExists /* && append */ {
			index, err := findSectionId(title, sectionTitle)
			if err != nil {
				return err
			}
			parameters["section"] = index
		} else {
			parameters["section"] = "new"
			parameters["sectiontitle"] = sectionTitle
		}
	}

	if append {
		convo = "\n\n" + convo // Prepend some newlines so that he gets formatted properly
		delete(parameters, "text")
		parameters["appendtext"] = convo
		parameters["summary"] = fmt.Sprintf("Grab publishToWiki append section %s", sectionTitle)
	}

	// Make the request.
	return w.Edit(parameters)
}

func uploadToWiki(path string) (err error) {
	// Open the file
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get csrf token
	csrfToken, err := w.GetToken(mwclient.CSRFToken)
	if err != nil {
		return err
	}

	fmt.Println(csrfToken)

	// Create a new HTTP request with the CSRF token
	url := config.WikiURL // Replace with your MediaWiki API endpoint
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("token", csrfToken)
	writer.WriteField("action", "upload")
	writer.WriteField("format", "json")
	writer.WriteField("filename", "file.jpg")
	writer.WriteField("text", "Description of the uploaded file.")
	part, err := writer.CreateFormFile("file", file.Name())
	if err != nil {
		return err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}
	writer.Close()


	// Make the POST request to upload the file
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Process the response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upload failed with status code %d", resp.StatusCode)
	}

	// You might want to further process the response here

	fmt.Println(resp)
	
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return err
    }
    fmt.Println("Response Body:", string(responseBody))


	return nil
}

func getArticleURL(title string) (url string, missing bool, err error) {
	newArticleParameters := map[string]string{
		"action": "query",
		"format": "json",
		"titles": title,
		"prop":   "info",
		"inprop": "url",
	}

	newArticle, err := w.Get(newArticleParameters)
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
func sectionExists(title string, section string) (exists bool, err error) {
	sectionQueryParameters := map[string]string{
		"format": "json",
		"action": "parse",
		"page":   title,
		"prop":   "sections",
	}

	sectionQuery, err := w.Get(sectionQueryParameters)
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
func findSectionId(title string, section string) (id string, err error) {
	sectionQueryParameters := map[string]string{
		"format": "json",
		"action": "parse",
		"page":   title,
		"prop":   "sections",
	}

	sectionQuery, err := w.Get(sectionQueryParameters)
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
