package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

func tryRefresh(clientID, clientSecret, refreshToken string) {
	target := "https://oauth2.googleapis.com/token"

	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	}

	req, err := http.NewRequest("POST", target, bytes.NewBufferString(form.Encode()))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	displayID := clientID
	if len(displayID) > 15 {
		displayID = displayID[:15]
	}
	fmt.Printf("ID: %s, SECRET: %s -> STATUS: %d\n", displayID, clientSecret, resp.StatusCode)
	fmt.Printf("BODY: %s\n\n", string(body))
}

func main() {
	refreshToken := os.Getenv("ANTIGRAVITY_REFRESH_TOKEN")
	if refreshToken == "" {
		fmt.Println("Error: ANTIGRAVITY_REFRESH_TOKEN environment variable is not set")
		os.Exit(1)
	}
	
	clientIDsStr := os.Getenv("ANTIGRAVITY_CLIENT_IDS")
	var clientIDs []string
	if clientIDsStr != "" {
		clientIDs = strings.Split(clientIDsStr, ",")
	} else {
		fmt.Println("Error: ANTIGRAVITY_CLIENT_IDS environment variable is not set (comma-separated list)")
		os.Exit(1)
	}

	secretsStr := os.Getenv("ANTIGRAVITY_CLIENT_SECRETS")
	var secrets []string
	if secretsStr != "" {
		secrets = strings.Split(secretsStr, ",")
	} else {
		secrets = []string{""}
	}

	for _, cid := range clientIDs {
		for _, sec := range secrets {
			tryRefresh(strings.TrimSpace(cid), strings.TrimSpace(sec), refreshToken)
		}
	}
}
