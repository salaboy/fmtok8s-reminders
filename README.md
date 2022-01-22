# FMTOK8S Reminders and Notification Services (Go)

This project implements a [CloudEvents](http://cloudevents.io)-based reminder service. 
This service allows your services or applications to schedule reminders that will emit CloudEvents when the reminder is due.

You can configure where the CloudEvents will be sent by exporting an environment variable called `SINK`, which by default will send events to `http://localhost:8080/events`
In most cases, the reminders service will emit CloudEvents to a broker and not directly to a specific service, hence Knative Eventing is recommended for this use.

**Note**: This service avoids the use of the Kubernetes Control Plane, hence it doesn't define any CRD, or require any kind of special permissions to run inside a cluster,
in contrast with the services like [PingSource provided by Knative](https://knative.dev/docs/eventing/sources/ping-source/).

Reminders JSON Object: 
```json
{
  "id": "<UUID>",
  "cronJobId": "<CronJOBID>",
  "name": "talk scheduled",
  "type": "email-reminder",
  "when": "@every 1s",
  "forWho": "salaboy@mail.com",
  "data" : "{}"
}

```




This project expose the following REST endpoints
- GET `/reminders`: Get all schedule reminders
- POST `/reminders`: Create a new Reminder
- POST `/reminders/{id}/dismiss`:  
- POST `/reminders/{id}/snooze`: 
- POST `/events`: Consume CloudEvents and based on the type can schedule different reminders


This service also emits the following Events
- `ReminderCreated`
- `ReminderTriggered`

# Run and test

```json
go run main.go
```

or run with `ko` in Kubernetes:

```
ko apply -f config/
```

Schedule a notification every 1 second.
```
 curl -X POST -d '{"when":"@every 1s", "type":"email-notification", "forWho":"salaboy@gmail.com", "data": "important email about conference"}' http://localhost:8080/reminders 
```
Get all scheduled notifications
```
curl  http://localhost:8080/reminders 
```
Remove a Reminder by sending the Reminder Id and the CronJobID
```
curl -X DELETE -d '{"id":"770f40fe-79cd-11ec-a8f5-367ddaa504e1","cronJobId":"1"}' http://localhost:8080/reminders
```