package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	apipb "remote-risk-engine-plugin-demo/api/gen/workshop/v1"

	"google.golang.org/protobuf/encoding/protojson"
)

type Decision int8

const (
	DecisionUnknown     Decision = 0
	DecisionDeny                 = 1
	DecicionDenyMalware          = 2
	DecisionAllow                = 3
	DecisionTimeout              = 4
	DecisionError                = 5
)

type AuthzRequest struct {
	TXID      string                 `json:"txid"`
	Blockable *apipb.BinaryBlockable `json:"blockable"`
	StartTime time.Time              `json:"date"`
	GoodUntil time.Time              `json:"good_until"`
}

// Explanation of the decision and if this is JSON or not.
type Explanation struct {
	Msg  string `json:"msg"`
	URL  string `json:"url"`
	Json bool   `json:"is_json"`
}

// Response required from the plugin
type AuthzResponse struct {
	Decision    Decision     `json:"decision"`
	GoodUntil   time.Time    `json:"good_until"`
	Explanation *Explanation `json:"explanation"`
	Error       string       `json:"error"`
	PluginUUID  string       `json:"plugin_uuid"`
	TXID        string       `json:"txid"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()

	// Read in the BinaryBlockable from blockable.json file
	b, err := os.OpenFile("blockable.json", os.O_RDONLY, 0)
	if err != nil {
		log.Fatal(err)
	}
	blockable := &apipb.BinaryBlockable{}
	bb, err := io.ReadAll(b)

	if err != nil {
		log.Fatal(err)
	}

	err = protojson.Unmarshal(bb, blockable)
	if err != nil {
		log.Fatal(err)
	}

	// Make a new AuthzRequest
	req := &AuthzRequest{
		TXID:      "1234567890",
		Blockable: blockable,
		StartTime: time.Now(),
		GoodUntil: time.Now().Add(time.Minute * 5),
	}

	fmt.Println("Authz Blockable: ", blockable)

	// Make the new request to the server (HTTP Post with extra headers)
	// Convert the AuthzRequest to JSON
	reqBody, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	// Create a new HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST",
		"http://localhost:8888/", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Fatal(err)
	}

	// Set the content type and custom headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-KEY", "my-secret-api-key")

	// Make the HTTP request
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Read and log the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var authzResp AuthzResponse
	err = json.Unmarshal(respBody, &authzResp)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("AuthzResponse: ", string(respBody))

}
