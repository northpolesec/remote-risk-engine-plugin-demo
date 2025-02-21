package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/acme/autocert"

	apipb "remote-risk-engine-plugin-demo/api/gen/workshop/v1"
)

var (
	apiKeyHeader = "X-API-KEY"
	validAPIKey  = "my-secret-api-key"
)

const (
	iTunesURL = "https://itunes.apple.com/lookup?bundleId=%s&entity=macSoftware&country=us&limit=1"
)

type AuthzRequest struct {
	TXID      string                 `json:"txid"`
	Blockable *apipb.BinaryBlockable `json:"blockable"`
	StartTime time.Time              `json:"date"`
}

type Decision int8

const (
	DecisionUnknown     Decision = 0
	DecisionDeny                 = 1
	DecicionDenyMalware          = 2
	DecisionAllow                = 3
	DecisionTimeout              = 4
	DecisionError                = 5
)

// Explanation of the decision and if this is JSON or not.
type Explanation struct {
	Msg  string `json:"msg"`
	URL  string `json:"url"`
	Json bool   `json:"is_json"`
}

// Response required from the plugin
type AuthzResponse struct {
	Decision    Decision     `json:"decision"`
	TXID        string       `json:"txid"`
	GoodUntil   time.Time    `json:"good_until"`
	Explanation *Explanation `json:"explanation"`
	Error       string       `json:"error"`
	PluginUUID  string       `json:"plugin_uuid"`
}

func main() {
	useHTTPS := flag.Bool("https", false, "Enable HTTPS")
	flag.Parse()

	http.HandleFunc("/", EvaluateHandler)

	if *useHTTPS {
		// Configure autocert for HTTPS
		certManager := autocert.Manager{
			Prompt: autocert.AcceptTOS,
			Cache:  autocert.DirCache(".certs"),
		}

		httpsServer := &http.Server{
			Addr:      ":443",
			Handler:   nil,
			TLSConfig: certManager.TLSConfig(),
		}

		// Start the server in a goroutine
		go func() {
			log.Println("Starting HTTPS server...")
			log.Fatal(httpsServer.ListenAndServeTLS("", ""))
		}()

		// Redirect HTTP to HTTPS
		http.ListenAndServe(":80", certManager.HTTPHandler(nil))
	} else {
		log.Println("Starting HTTP server...")
		http.ListenAndServe(":8888", nil)
	}
}

type Result struct {
	BundleID               string `json:"bundleId"`
	Company                string `json:"sellerName"`
	CompanyURL             string `json:"sellerUrl"`
	InitialReleaseDate     string `json:"releaseDate"`
	CurrentReleaseDate     string `json:"currentVersionReleaseDate"`
	RatingsCountForVersion int64  `json:"userRatingCountForVersion"`
}

type iTunesQueryResult struct {
	Results []Result `json:"results"`
}

// CheckiTunesStore checks the iTunesStore API to see if the binary's been
// available on the app store for more than 730 hours roughly 30.5 days.
func CheckiTunesStore(bundleID string) (bool, time.Time, string, error) {
	url := fmt.Sprintf(iTunesURL, bundleID)
	resp, err := http.Get(url)
	if err != nil {
		return false, time.Time{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, time.Time{}, "", fmt.Errorf("failed to search for %s: %s", bundleID, resp.Status)
	}

	// Get the body and unmarshal the JSON
	var qRes iTunesQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&qRes); err != nil {
		return false, time.Time{}, "", err
	}

	if len(qRes.Results) == 0 {
		return false, time.Time{}, "No results", nil
	}

	res := qRes.Results[0]
	// Check the release date vs. the current release date
	firstRelease, err := time.Parse(time.RFC3339, res.InitialReleaseDate)
	if err != nil {
		return false, time.Time{}, "", err
	}
	lastRelease, err := time.Parse(time.RFC3339, res.CurrentReleaseDate)
	if err != nil {
		return false, time.Time{}, "", err
	}

	// Check if the app has been on the App Store for less than a month
	if lastRelease.Sub(firstRelease) < 730*time.Hour {
		deltaUntilGood := 730*time.Hour - lastRelease.Sub(firstRelease)
		goodUntil := time.Now().Add(deltaUntilGood)
		return true, goodUntil, "App has been on the App Store for less than a month", nil
	} else {
		// Set this to a date in the distant future since this application will
		// always have been on the app store for longer than 30 days.
		return true, time.Date(3000, 12, 25, 0, 0, 0, 0, time.UTC),
			"App has been on the App Store for more than a month", nil
	}
}

func EvaluateHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Received request")
	// Check for API key
	apiKey := r.Header.Get(apiKeyHeader)
	if apiKey != validAPIKey {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	authzReq := &AuthzRequest{}
	if err := json.Unmarshal(data, authzReq); err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	log.Println("req: ", authzReq.Blockable.GetTeamId())

	// Check time.
	w.Header().Set("Content-Type", "application/json")

	authzResp := &AuthzResponse{
		Decision:    DecisionUnknown,
		PluginUUID:  "271b581e-498c-4ef0-95f2-57cdc6330e22",
		TXID:        authzReq.TXID,
		Explanation: &Explanation{Msg: "Unknown"},
	}

	// Check if the binary is from the iTunes Store.
	if authzReq.Blockable == nil {
		authzResp.Decision = DecisionError
		authzResp.Error = "Blockable is nil"
		json.NewEncoder(w).Encode(authzResp)
		log.Println("Blockable is nil")
		return
	}

	signingCert := authzReq.Blockable.GetSignedBy()
	log.Println("Signing Cert:", signingCert)

	if signingCert.GetSignedBy() != "53fd008278e5a595fe1e908ae9c5e5675f26243264a5a6438c023e3ce2870760" {
		authzResp.Decision = DecisionAllow
		authzResp.Explanation.Msg = "Binary is not not from the app store"
		json.NewEncoder(w).Encode(authzResp)
		log.Println("Binary is not from the app store")
		return
	}
	// Fetch the json from the iTunes Store Search API and check if the first
	// release date vs. the current release date is less than a month old.
	bundleID := authzReq.Blockable.GetSigningId()

	// strip the TeamID: prefix from the bundleID
	if strings.HasPrefix(bundleID, "platform:") {
		bundleID = strings.TrimPrefix(bundleID, "platform:")
	} else if len(bundleID) > 11 {
		bundleIDParts := strings.Split(bundleID, ":")
		if len(bundleIDParts) < 2 {
			authzResp.Decision = DecisionError
			authzResp.Error = "Invalid bundle ID"
			json.NewEncoder(w).Encode(authzResp)
			return
		}

		bundleID = strings.Join(bundleIDParts[1:], ":")
	} else {
		authzResp.Decision = DecisionError
		authzResp.Error = "Invalid bundle ID"
		json.NewEncoder(w).Encode(authzResp)
		return
	}

	match, goodUntil, msg, err := CheckiTunesStore(bundleID)

	if err != nil {
		authzResp.GoodUntil = time.Time{}
		authzResp.Decision = DecisionError
		authzResp.Error = err.Error()
		json.NewEncoder(w).Encode(authzResp)
		return
	}

	// If a rule matched then we should deny the event.
	authzResp.GoodUntil = goodUntil

	if match {
		authzResp.Decision = DecisionDeny
		authzResp.Explanation = &Explanation{
			Msg: msg,
		}
		json.NewEncoder(w).Encode(authzResp)
		return
	}

	authzResp.Decision = DecisionAllow
	authzResp.Explanation = &Explanation{
		Msg: msg,
	}

	log.Println("Decision:", authzResp.Decision)

	json.NewEncoder(w).Encode(authzResp)
}
