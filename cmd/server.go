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
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	apipb "buf.build/gen/go/northpolesec/workshop-api/protocolbuffers/go/workshop/v1"
)

var (
	apiKeyHeader = "X-API-KEY"
	validAPIKey  = "my-secret-api-key"

	// This is a date in the distant future that ensures workshop can cache the
	// result forever.
	forever = time.Date(3000, 12, 25, 0, 0, 0, 0, time.UTC)
)

const (
	iTunesURL          = "https://itunes.apple.com/lookup?bundleId=%s&entity=macSoftware&country=us&limit=1"
	appStoreCertSha256 = "53fd008278e5a595fe1e908ae9c5e5675f26243264a5a6438c023e3ce2870760"

	oneMonth = 30 * 24 * time.Hour
)

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
		http.ListenAndServe("127.0.0.1:8888", nil)
	}
}

// Result is the struct that represents the JSON response from the iTunes Store
type Result struct {
	BundleID               string `json:"bundleId"`
	Company                string `json:"sellerName"`
	CompanyURL             string `json:"sellerUrl"`
	TrackViewURL           string `json:"trackViewUrl"`
	InitialReleaseDate     string `json:"releaseDate"`
	CurrentReleaseDate     string `json:"currentVersionReleaseDate"`
	RatingsCountForVersion int64  `json:"userRatingCountForVersion"`
}

// iTunesQueryResult is the struct that represents the JSON response from the
// iTunes Store API. It contains an array of Result structs though by default we
// limit it to one.
type iTunesQueryResult struct {
	Results []Result `json:"results"`
}

// CheckiTunesStore checks the iTunesStore API to see if the binary's been
// available on Apple's App Store for more than 30 days.
func CheckiTunesStore(bundleID string) (bool, string, time.Time, string, error) {
	url := fmt.Sprintf(iTunesURL, bundleID)
	resp, err := http.Get(url)
	if err != nil {
		return false, "", time.Time{}, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, "", time.Time{}, "", fmt.Errorf("failed to search for %s: %s", bundleID, resp.Status)
	}

	// Get the body and unmarshal the JSON
	var qRes iTunesQueryResult
	if err := json.NewDecoder(resp.Body).Decode(&qRes); err != nil {
		return false, "", time.Time{}, "", err
	}

	if len(qRes.Results) == 0 {
		return false, "", time.Time{}, "No results", nil
	}

	res := qRes.Results[0]
	// Check the release date vs. the current release date
	firstRelease, err := time.Parse(time.RFC3339, res.InitialReleaseDate)
	if err != nil {
		return false, "", time.Time{}, "", err
	}

	// Check if the app has been on the App Store for less than a month
	now := time.Now()
	if now.Sub(firstRelease) < oneMonth {
		deltaUntilGood := oneMonth - now.Sub(firstRelease)
		goodUntil := time.Now().Add(deltaUntilGood)
		msg := fmt.Sprintf("App (%s) has been on the App Store for less than a month", bundleID)
		return true, res.TrackViewURL, goodUntil, msg, nil
	} else {
		msg := fmt.Sprintf("App (%s) has been on the App Store for more than a month", bundleID)
		// Set this to a date in the distant future since this application will
		// always have been on the app store for longer than 30 days.
		return false, res.TrackViewURL, forever, msg, nil
	}
}

func EvaluateHandler(w http.ResponseWriter, r *http.Request) {
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

	authzReq := &apipb.PluginAuthzRequest{}
	if err := protojson.Unmarshal(data, authzReq); err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	// Check time.
	w.Header().Set("Content-Type", "application/json")

	authzResp := &apipb.PluginAuthzResponse{
		Decision:    apipb.Decision_DECISION_ERROR,
		PluginUuid:  "271b581e-498c-4ef0-95f2-57cdc6330e22",
		TxId:        authzReq.TxId,
		Explanation: &apipb.Explanation{Message: "Unknown"},
	}

	// Check if the binary is from the iTunes Store.
	if authzReq.Blockable == nil {
		authzResp.Decision = apipb.Decision_DECISION_ERROR
		authzResp.Error = "Blockable is nil"
		json.NewEncoder(w).Encode(authzResp)
		log.Println("Blockable is nil")
		return
	}

	signingCert := authzReq.Blockable.GetSignedBy()

	if signingCert.GetSignedBy() != appStoreCertSha256 {
		authzResp.Decision = apipb.Decision_DECISION_ALLOW
		authzResp.Explanation.Message = "Binary is not not from the app store"
		json.NewEncoder(w).Encode(authzResp)
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
			authzResp.Decision = apipb.Decision_DECISION_ERROR
			authzResp.Error = "Malformed signing ID"
			json.NewEncoder(w).Encode(authzResp)
			return
		}

		bundleID = strings.Join(bundleIDParts[1:], ":")
	} else {
		authzResp.Decision = apipb.Decision_DECISION_ERROR
		authzResp.Error = "Invalid bundle ID"
		json.NewEncoder(w).Encode(authzResp)
		return
	}

	match, storeURL, goodUntil, msg, err := CheckiTunesStore(bundleID)

	if err != nil {
		authzResp.GoodUntil = timestamppb.New(time.Time{})
		authzResp.Decision = apipb.Decision_DECISION_ERROR
		authzResp.Error = err.Error()
		json.NewEncoder(w).Encode(authzResp)
		return
	}

	// If a rule matched then we should deny the event.
	authzResp.GoodUntil = timestamppb.New(goodUntil)
	authzResp.Explanation.Url = storeURL
	authzResp.Explanation.Message = msg

	if match {
		authzResp.Decision = apipb.Decision_DECISION_DENY
	} else {
		authzResp.Decision = apipb.Decision_DECISION_ALLOW
	}

	log.Println("Binary: ", authzReq.Blockable.FileName)
	log.Println("Decision:", authzResp.Decision)
	log.Println("Team ID: ", authzReq.Blockable.GetTeamId())
	log.Println("App Store URL: ", storeURL)
	log.Println("Good until: ", goodUntil)
	log.Println()
	log.Println()

	json.NewEncoder(w).Encode(authzResp)
}
