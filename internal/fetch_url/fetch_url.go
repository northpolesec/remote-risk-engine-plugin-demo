package fetch_url

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// FetchURL fetches the URL and returns the body
// Send GET request
func FetchURLAsString(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: %s, status code: %d",
			url, resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func FetchFieldFromURL(url, selector string) (string, error) {
	// Send GET request
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch URL: %s, status code: %d",
			url, resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return "", err
	}

	// Find and extract the ratings text
	ratingsText := doc.Find(selector).Text()

	// Extract the actual ratings count
	parts := strings.Split(ratingsText, "•")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[1]), nil
	}

	return "", fmt.Errorf("rating count not found")
}
