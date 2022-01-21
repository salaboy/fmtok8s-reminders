package main

import (
	"context"
	"encoding/json"
	"fmt"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/robfig/cron/v3"
	"log"
	"net/http"
	"os"
	"strconv"
)

var SINK = getEnv("SINK", "http://localhost:8080/events")

var SERVER_PORT = getEnv("SERVER_PORT", ":8080")

type cronJobsRunner struct {
	// The cron job runner
	cron cron.Cron

	// client sends cloudevents.
	Client cloudevents.Client

	reminders []Reminder
}

func NewCronJobsRunner(ceClient cloudevents.Client,
	opts ...cron.Option) *cronJobsRunner {

	return &cronJobsRunner{
		cron:   *cron.New(opts...),
		Client: ceClient,
	}
}

func (cjr *cronJobsRunner) AddSchedule(ctx context.Context, reminder *Reminder) cron.EntryID {
	id, _ := cjr.cron.AddFunc(reminder.When, cjr.cronTick(ctx, reminder))
	reminder.CronJobId = fmt.Sprintf("%v", id)
	cjr.reminders = append(cjr.reminders, *reminder)
	return id
}

func (cjr *cronJobsRunner) cronTick(ctx context.Context, reminder *Reminder) func() {
	return func() {
		//TODO: Check Templates, based on the defined templates, decide which CloudEvent emit
		if reminder.Type == "email-notification" {
			cjr.SendEmailNotification(reminder)
		}
	}
}

func (cjr *cronJobsRunner) RemoveSchedule(id string) {
	atoi, _ := strconv.Atoi(id)
	entryID := cron.EntryID(atoi)
	cjr.cron.Remove(entryID)
}

func (cjr *cronJobsRunner) Start(stopCh <-chan struct{}) {
	cjr.cron.Start()
	<-stopCh
}

func (cjr *cronJobsRunner) Stop() {
	ctx := cjr.cron.Stop() // no more ticks
	if ctx != nil {
		// Wait for all jobs to be done.
		<-ctx.Done()
	}
}

type Reminder struct {
	ID        string `json:"id"`
	CronJobId string `json:"cronJobId"`
	Type      string `json:"type"`
	ForWho    string `json:"forWho"`
	When      string `json:"when"`
	Data      string `json:"data"`
}

func ConsumeCloudEventHandler(ctx context.Context, event cloudevents.Event) {
	reminder := Reminder{}
	fmt.Printf("Got an Event: %s", event)
	json.Unmarshal(event.Data(), &reminder)

	fmt.Printf("Reminder: %v\n", reminder.ForWho)
	fmt.Printf("Reminder Data: %v\n", reminder.Data)

}

func (cjr *cronJobsRunner) SendEmailNotification(reminder *Reminder) {
	log.Printf("Email CloudEvent Sent to: %v with body: %v\n", reminder.ForWho, reminder.Data)

	// Create an Event.
	event := cloudevents.NewEvent()
	event.SetSource("reminders")
	event.SetType("reminders.EmailRequested")

	event.SetData(cloudevents.ApplicationJSON, &reminder)

	log.Printf("CloudEvent: %v\n", event )
	// Set a target.
	ctx := cloudevents.ContextWithTarget(context.Background(), SINK)
	// Send that Event.

	if result := cjr.Client.Send(ctx, event); !cloudevents.IsACK(result) {
		log.Fatalf("failed to send (!IsACK), %v", result)
	}

}

func NewReminderHandler(writer http.ResponseWriter, request *http.Request) {
	var r Reminder
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&r); err != nil {
		respondWithError(writer, http.StatusBadRequest, "Invalid request payload")
		return
	}
	newUUID, _ := uuid.NewUUID()
	r.ID = newUUID.String()
	defer request.Body.Close()
	ctx := context.Background()
	runner.AddSchedule(ctx, &r)

}

func DeleteReminderHandler(writer http.ResponseWriter, request *http.Request) {
	var r Reminder
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&r); err != nil {
		respondWithError(writer, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer request.Body.Close()
	log.Printf("Removing Reminder with ID: %v and CronJobID: %v\n", r.ID, r.CronJobId)
	runner.RemoveSchedule(r.CronJobId)
}

func GetRemindersHandler(writer http.ResponseWriter, request *http.Request) {
	entries := runner.cron.Entries()

	log.Printf("Printing all schedule Reminders\n")
	for index, element := range entries {
		log.Printf("Index: %v - Value: %v \n", index, element)
	}

	respondWithJSON(writer, http.StatusOK, runner.reminders)

}

var runner *cronJobsRunner
var logger *log.Logger

func main() {
	logger = log.New(os.Stdout, "", 0)

	ctx := context.Background()
	p, err := cloudevents.NewHTTP()
	if err != nil {

		logger.Fatalf("failed to create protocol: %s", err.Error())
	}

	h, err := cloudevents.NewHTTPReceiveHandler(ctx, p, ConsumeCloudEventHandler)
	if err != nil {
		logger.Fatalf("failed to create handler: %s", err.Error())
	}

	// Use a gorilla mux implementation for the overall http handler.
	router := mux.NewRouter()

	router.HandleFunc("/reminders", NewReminderHandler).Methods("POST")
	router.HandleFunc("/reminders", GetRemindersHandler).Methods("GET")
	router.HandleFunc("/reminders", DeleteReminderHandler).Methods("DELETE")

	router.Handle("/events", h)

	logger.Printf("Starting Reminders Service...\n")

	ceClient, err := cloudevents.NewClient(p, cloudevents.WithTimeNow(), cloudevents.WithUUIDs())
	if err != nil {
		log.Fatalf("failed to create client, %v", err)
	}

	runner = NewCronJobsRunner(ceClient, cron.WithSeconds())

	go func(ctx context.Context) {
		runner.Start(ctx.Done())
	}(ctx)

	logger.Printf("Will listen on %v\n", SERVER_PORT)
	if err := http.ListenAndServe(SERVER_PORT, router); err != nil {
		logger.Fatalf("unable to start http server, %s", err)
		runner.Stop()
	}

}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}
