package duckduckgo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	duckduckgotool "github.com/quka-ai/quka-ai/pkg/ai/tools/duckduckgo"
)

func TestSearch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	tool, err := duckduckgotool.NewTool(ctx, duckduckgo.RegionCN)
	if err != nil {
		t.Fatal(err)
	}

	// Create search request
	searchReq := &duckduckgo.TextSearchRequest{
		Query: "Golang programming development",
	}

	jsonReq, err := json.Marshal(searchReq)
	if err != nil {
		log.Fatalf("Marshal of search request failed, err=%v", err)
	}

	// Execute search
	resp, err := tool.InvokableRun(ctx, string(jsonReq))
	if err != nil {
		log.Fatalf("Search of duckduckgo failed, err=%v", err)
	}

	var searchResp duckduckgo.TextSearchResponse
	if err := json.Unmarshal([]byte(resp), &searchResp); err != nil {
		log.Fatalf("Unmarshal of search response failed, err=%v", err)
	}

	// Print results
	fmt.Println("Search Results:")
	fmt.Println("==============")
	for i, result := range searchResp.Results {
		fmt.Printf("\n%d. Title: %s\n", i+1, result.Title)
		fmt.Printf("   Link: %s\n", result.URL)
		fmt.Printf("   Description: %s\n", result.Summary)
	}
	fmt.Println("")
	fmt.Println("==============")
}
