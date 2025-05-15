package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// send post request to server
func TestApp(t *testing.T) {

	jsonData, err := json.Marshal(map[string]string{
		"source_image":       "hub.hzh.sealos.run/ns-idau55fm/aizhinengpingtai:kcwfj-2025-05-07-064103",
		"target_image":       "hub.hzh.sealos.run/ns-idau55fm/aizhinengpingtai:squashed-l2",
		"squash_layer_count": "2",
	})
	if err != nil {
		t.Fatalf("Failed to marshal JSON data: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:8080/squash", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	t.Logf("Response: %s", string(body))
}
