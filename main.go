package main

import (
	"encoding/json"
	"fmt"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var api = slack.New("xoxb-324896932822-1681656151255-sDAfMutG1atKEHre8WmlF0H9", slack.OptionDebug(true))

func main() {
	http.HandleFunc("/mention", func(w http.ResponseWriter, r *http.Request){
		fmt.Printf("Got mention event!\n")
		body, err := ioutil.ReadAll(r.Body)
		eventsAPIEvent, err := slackevents.ParseEvent(body, slackevents.OptionNoVerifyToken())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if eventsAPIEvent.Type == slackevents.URLVerification {
			var r *slackevents.ChallengeResponse
			err := json.Unmarshal([]byte(body), &r)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text")
			w.Write([]byte(r.Challenge))
		}
		if eventsAPIEvent.Type == slackevents.CallbackEvent {
			fmt.Printf("Got callback event!\n")
			innerEvent := eventsAPIEvent.InnerEvent
			switch ev := innerEvent.Data.(type) {
			case *slackevents.AppMentionEvent:
				user, err := api.GetUserInfo(ev.User)
				//postMessageParameters := slack.NewPostMessageParameters()
				//postMessageParameters.LinkNames = 1
				//msgOption := slack.MsgOptionPostMessageParameters(postMessageParameters)
				if user == nil || err != nil {
					fmt.Printf("User is nil")
					api.PostMessage(ev.Channel,
						slack.MsgOptionText("Hello, whoever you are", false),
						//msgOption,
					)
				}
				api.PostMessage(ev.Channel,
					slack.MsgOptionText(fmt.Sprintf("Hello, <@%s>", user.ID), false),
					//msgOption,
				)
			}
		}
	})

	http.HandleFunc("/command", func(w http.ResponseWriter, r *http.Request){
		slashCommand, err := slack.SlashCommandParse(r)
		if err != nil {
			log.Fatalf("Got err %+v\n", err)
		}
		fmt.Printf("Command: %s\n", slashCommand.Command)
		if slashCommand.Command == "/annoygregg" {
			go AnnoyGregg(slashCommand)
		}
	})

	http.HandleFunc("/interact", func(w http.ResponseWriter, r *http.Request){
		slashCommand, err := slack.SlashCommandParse(r)
		if err != nil {
			log.Fatalf("Got err %+v\n", err)
		}
		fmt.Printf("Command: %s\n", slashCommand.Command)


		modalRequest := generateModal()
		_, err = api.OpenView(slashCommand.TriggerID, modalRequest)
		if err != nil {
			fmt.Printf("Error opening view: %s", err)
		}

		//_, _, err = api.PostMessage(slashCommand.ChannelID,
		//	slack.MsgOptionBlocks(
		//		//slack.NewContextBlock(
		//		//	"context",
		//		//	slack.NewTextBlockObject(
		//		//		slack.PlainTextType,
		//		//		"test",
		//		//		false,
		//		//		false,
		//		//	),
		//		//),
		//		slack.NewInputBlock(
		//			"input", //	A string acting as a unique identifier for a block. If not specified, one will be generated. Maximum length for this field is 255 characters. block_id should be unique for each message and each iteration of a message. If a message is updated, use a new block_id.
		//			slack.NewTextBlockObject(
		//				slack.PlainTextType,
		//				"label_test",
		//				false,
		//				false,
		//			),
		//			slack.NewPlainTextInputBlockElement(
		//				slack.NewTextBlockObject(
		//					slack.PlainTextType,
		//					"placeholder_tet",
		//					false,
		//					false,
		//					),
		//				"test_input", // This is used to identify the data in the response from the form submission
		//			),
		//		),
		//	),
		//	slack.MsgOptionText(fmt.Sprintf("Received command `/interact`"), false),
		//
		//)

		//if err != nil {
		//	log.Fatalf("Got err: %v", err)
		//}

	})

	http.HandleFunc("/interactive-submit", func(w http.ResponseWriter, r *http.Request){
		var i slack.InteractionCallback

		err := json.Unmarshal([]byte(r.FormValue("payload")), &i)
		if err != nil {
			fmt.Printf(err.Error())
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Note there might be a better way to get this info, but I figured this structure out from looking at the json response
		firstName := i.View.State.Values["First Name"]["firstName"].Value
		lastName := i.View.State.Values["Last Name"]["lastName"].Value

		msg := fmt.Sprintf("Hello %s %s, nice to meet you!", firstName, lastName)

		log.Printf("%+v", i)
		_, _, err = api.PostMessage(i.User.ID,
			slack.MsgOptionText(msg, false),
			)

		if err != nil {
			fmt.Printf(err.Error())
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	})

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Got Error: %v", err)
	}
}

func generateModal() slack.ModalViewRequest {
	// Create a ModalViewRequest with a header and two inputs
	titleText := slack.NewTextBlockObject("plain_text", "My App", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "Close", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "Submit", false, false)

	headerText := slack.NewTextBlockObject("mrkdwn", "Please enter your name", false, false)
	headerSection := slack.NewSectionBlock(headerText, nil, nil)

	firstNameText := slack.NewTextBlockObject("plain_text", "First Name", false, false)
	firstNamePlaceholder := slack.NewTextBlockObject("plain_text", "Enter your first name", false, false)
	firstNameElement := slack.NewPlainTextInputBlockElement(firstNamePlaceholder, "firstName")
	// Notice that blockID is a unique identifier for a block
	firstName := slack.NewInputBlock("First Name", firstNameText, firstNameElement)

	lastNameText := slack.NewTextBlockObject("plain_text", "Last Name", false, false)
	lastNamePlaceholder := slack.NewTextBlockObject("plain_text", "Enter your first name", false, false)
	lastNameElement := slack.NewPlainTextInputBlockElement(lastNamePlaceholder, "lastName")
	lastName := slack.NewInputBlock("Last Name", lastNameText, lastNameElement)

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			headerSection,
			firstName,
			lastName,
		},
	}

	var modalRequest slack.ModalViewRequest
	modalRequest.Type = slack.ViewType("modal")
	modalRequest.Title = titleText
	modalRequest.Close = closeText
	modalRequest.Submit = submitText
	modalRequest.Blocks = blocks
	return modalRequest
}



func AnnoyGregg(s slack.SlashCommand) {
	api.PostMessage(s.ChannelID,
			slack.MsgOptionText(fmt.Sprintf("Received command `/annoygregg`"), false),
			//msgOption,
	)

	var greggUserId string
	users, err := api.GetUsers()
	if err != nil {
		log.Fatalf("Fatal Error getting users %v", users)
	}
	for _, user := range users {
		if user.Profile.DisplayNameNormalized == "Gregg" {
			greggUserId = user.ID
		}
	}

	if greggUserId == "" {
		log.Fatalf("Could not find Gregg")
	}

	ticker := time.NewTicker(1 * time.Second)
	for i := 0; i< 10; i++ {
		select {
		case <- ticker.C:
			api.PostMessage(s.ChannelID,
				slack.MsgOptionText(fmt.Sprintf("Hi <@%s> :wave:", greggUserId), false),
			)
		}
	}
}