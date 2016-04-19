package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/google/go-github/github"
	"github.com/julienschmidt/httprouter"
	"github.com/zorkian/go-datadog-api"
)

type GithubEvent struct {
	Repo             *github.Repository
	EventType        string
	DeploymentStatus *github.DeploymentStatus
	Deployment       *github.Deployment
}

func main() {
	route := httprouter.New()
	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	route.POST("/webhook", DeploymentHandler)

	fmt.Println("Starting server on :" + port)

	http.ListenAndServe(":"+port, route)
}

func DeploymentHandler(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
	event_type := req.Header.Get("X-Github-Event")
	event, err := GetEvent(req, event_type)

	if err != nil {
		panic(err)
	}

	datadog_client := GetDataDogClient()
	datadog_event := GetDatadogEvent(event)

	_, datadog_err := datadog_client.PostEvent(datadog_event)
	if err != nil {
		fmt.Print(datadog_err)
	}
}

func GetEvent(req *http.Request, event_type string) (GithubEvent, error) {
	switch event_type {
	case "deployment":
		return decodeDeploymentEvent(req), nil
	case "deployment_status":
		return decodeDeploymentStatusEvent(req), nil
	}

	return GithubEvent{}, errors.New("Error: no matched event type")
}

func GetDatadogEvent(event GithubEvent) *datadog.Event {
	repoName := *event.Repo.FullName + ":" + *event.Deployment.SHA
	status := event.DeploymentStatus
	switch status {
	case nil:
		return &datadog.Event{
			Title: "Deployment of " + repoName + " started.",
		}
	default:
		return &datadog.Event{
			Title: "Deployment of " + repoName + " is " + *status.State,
			Text:  "Status: " + "[" + *status.State + "](" + *status.TargetURL + ")",
		}
	}
}

func GetDataDogClient() *datadog.Client {
	api_key := os.Getenv("DATADOG_API_KEY")
	app_key := os.Getenv("DATADOG_APP_KEY")

	client := datadog.NewClient(api_key, app_key)
	return client
}

func decodeDeploymentStatusEvent(req *http.Request) GithubEvent {
	decoder := json.NewDecoder(req.Body)

	var event github.DeploymentStatusEvent

	err := decoder.Decode(&event)

	if err != nil {
		fmt.Print(err)
	}

	github_event := GithubEvent{
		Repo:             event.Repo,
		EventType:        "DeploymentStatus",
		DeploymentStatus: event.DeploymentStatus,
		Deployment:       event.Deployment,
	}

	return github_event
}

func decodeDeploymentEvent(req *http.Request) GithubEvent {
	decoder := json.NewDecoder(req.Body)

	var event github.DeploymentEvent

	err := decoder.Decode(&event)

	if err != nil {
		fmt.Print(err)
	}

	github_event := GithubEvent{
		Repo:       event.Repo,
		EventType:  "DeploymentStatus",
		Deployment: event.Deployment,
	}

	return github_event
}
