package main

import (
	"encoding/json"
	"fmt"
	"github.com/davecgh/go-spew/spew"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/spf13/viper"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

var api *slack.Client

func main() {
	viper.SetConfigFile("./config.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Err reading in Config %v", err)
	}

	apiToken := viper.GetString("api_token")
	api = slack.New(apiToken, slack.OptionDebug(true))

	http.HandleFunc("/mention", func(w http.ResponseWriter, r *http.Request) {
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

	http.HandleFunc("/command", func(w http.ResponseWriter, r *http.Request) {
		slashCommand, err := slack.SlashCommandParse(r)
		if err != nil {
			log.Fatalf("Got err %+v\n", err)
		}
		fmt.Printf("Command: %s\n", slashCommand.Command)
		if slashCommand.Command == "/annoygregg" {
			go AnnoyGregg(slashCommand)
		}
	})

	http.HandleFunc("/interact", func(w http.ResponseWriter, r *http.Request) {
		slashCommand, err := slack.SlashCommandParse(r)
		if err != nil {
			log.Fatalf("Got err %+v\n", err)
		}
		fmt.Printf("Command: %s\n", slashCommand.Command)

		modalRequest := generateTestModal()
		_, err = api.OpenView(slashCommand.TriggerID, modalRequest)
		if err != nil {
			fmt.Printf("Error opening view: %s", err)
		}

	})

	http.HandleFunc("/interactive-submit", func(w http.ResponseWriter, r *http.Request) {
		var i slack.InteractionCallback

		err := json.Unmarshal([]byte(r.FormValue("payload")), &i)
		if err != nil {
			fmt.Printf(err.Error())
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		spew.Dump(i)
		switch i.View.CallbackID {
		case "test-modal":
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
			break
		case "incident-modal":
			handleIncidentModalSubmit(i)
			break
		default:
			log.Printf("Unknown Callback ID: %s", i.CallbackID)
		}
	})

	http.HandleFunc("/incident", func(w http.ResponseWriter, r *http.Request) {
		slashCommand, err := slack.SlashCommandParse(r)
		if err != nil {
			log.Printf("Got err %+v\n", err)
		}
		log.Printf("Command: %s\n", slashCommand.Command)

		modalRequest := generateIncidentModal()
		_, err = api.OpenView(slashCommand.TriggerID, modalRequest)
		if err != nil {
			log.Printf("Error opening view: %s", err)
		}
	})

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("Got Error: %v", err)
	}
}

func generateTestModal() slack.ModalViewRequest {
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
	modalRequest.CallbackID = "test-modal"
	return modalRequest
}

func generateIncidentModal() slack.ModalViewRequest {
	titleText := slack.NewTextBlockObject("plain_text", "Create Incident", false, false)
	closeText := slack.NewTextBlockObject("plain_text", "Cancel", false, false)
	submitText := slack.NewTextBlockObject("plain_text", "Submit", false, false)

	sectionText := slack.NewTextBlockObject("mrkdwn", "Please fill out the details of your incident", false, false)
	sectionBlock := slack.NewSectionBlock(sectionText, nil, nil)

	descriptionText := slack.NewTextBlockObject("plain_text", "Description", false, false)
	descriptionPlaceholder := slack.NewTextBlockObject("plain_text", "Saw an increase in map3 counts...", false, false)
	descriptionElement := slack.PlainTextInputBlockElement{
		Type:        slack.METPlainTextInput,
		ActionID:    "description",
		Placeholder: descriptionPlaceholder,
		Multiline:   true,
	}
	// Notice that blockID is a unique identifier for a block
	descriptionBlock := slack.NewInputBlock("Description", descriptionText, descriptionElement)

	priorityLabel := slack.NewTextBlockObject("plain_text", "Priority", false, false)
	priorityPlaceholder := slack.NewTextBlockObject("plain_text", "Select Priority", false, false)
	var priorityOptions []*slack.OptionBlockObject
	for i := 1; i <= 5; i++ {
		textString := fmt.Sprintf("%d", i)
		if i == 1 {
			textString += " - Lowest"
		} else if i == 5 {
			textString += " - Highest"
		}
		priorityOption := slack.OptionBlockObject{
			Text:  slack.NewTextBlockObject("plain_text", textString, false, false),
			Value: fmt.Sprintf("%d", i),
		}
		priorityOptions = append(priorityOptions, &priorityOption)
	}
	priorityElement := slack.NewOptionsSelectBlockElement("static_select", priorityPlaceholder, "priority", priorityOptions...)
	priorityBlock := slack.NewInputBlock("Priority", priorityLabel, priorityElement)

	blocks := slack.Blocks{
		BlockSet: []slack.Block{
			sectionBlock,
			descriptionBlock,
			priorityBlock,
		},
	}

	return slack.ModalViewRequest{
		Type:       slack.ViewType("modal"),
		Title:      titleText,
		Close:      closeText,
		Submit:     submitText,
		Blocks:     blocks,
		CallbackID: "incident-modal",
	}
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
	for i := 0; i < 10; i++ {
		select {
		case <-ticker.C:
			api.PostMessage(s.ChannelID,
				slack.MsgOptionText(fmt.Sprintf("Hi <@%s> :wave:", greggUserId), false),
			)
		}
	}
}

func handleIncidentModalSubmit(i slack.InteractionCallback) {
	log.Printf("Priority: %s", i.View.State.Values["Priority"]["priority"].SelectedOption.Value)
	log.Printf("Description: %s", i.View.State.Values["Description"]["description"].Value)
	log.Printf("Submitter: %s", i.User.Name)
	log.Printf(time.Now().String())
}
