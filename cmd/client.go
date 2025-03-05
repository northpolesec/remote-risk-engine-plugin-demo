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

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"

	apipb "buf.build/gen/go/northpolesec/workshop-api/protocolbuffers/go/workshop/v1"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	ctx := context.Background()

	// Read in the BinaryBlockable from blockable.json file
	b, err := os.OpenFile("./testdata/blockable.json", os.O_RDONLY, 0)
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
	now := time.Now()
	req := &apipb.PluginAuthzRequest{
		TxId:      "1234567890",
		Blockable: blockable,
		Timestamp: timestamppb.New(now),
		Deadline:  timestamppb.New(now.Add(time.Minute * 5)),
	}

	// Make the new request to the server (HTTP Post with extra headers)
	// Convert the AuthzRequest to JSON
	reqBody, err := protojson.Marshal(req)
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
	httpReq.Header.Set("X-API-Key", "sekrit")

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

	var authzResp apipb.PluginAuthzResponse
	err = json.Unmarshal(respBody, &authzResp)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(respBody))
}
