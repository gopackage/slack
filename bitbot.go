package main

import (
	"log"
	"os"

	"github.com/gopackage/slack/auth"
	"github.com/gopackage/slack/rtm"
)

const (
	// SlackAPIVersion is the version of the slack API being used
	SlackAPIVersion = "1"
	// BitbotVersion is the version of the bitbot library
	BitbotVersion = "0.0.1"
	// TokenKey is the name of the token environmental variable
	TokenKey = "BITBOT_TOKEN"
)

// Slack does stuff - nice huh?
func Slack() {
	// Pull in the auth token from the environment
	token := os.Getenv(TokenKey)
	if len(token) == 0 {
		// Bail
		log.Fatalln("Failed to read env variable", TokenKey)
	}
	verified, err := auth.VerifyToken(token)
	if err != nil {
		log.Fatalln("Failed to call verify API token", err)
	}
	if !verified {
		log.Fatalln("API token did not verify")
	}
	log.Println("token verified")
	log.Fatalln(rtm.DialAndListen(token))
}

func main() {
	log.Println("Bitbot", BitbotVersion)
	Slack()
}
