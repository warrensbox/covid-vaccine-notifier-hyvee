# HyVee only vacinne notifier
A lightweight application (lambda function) that notifies you when a vaccine appointment becomes available near you. Get notified on: 
- Slack
- Teams
- Email
- SMS

### Installation 
Install the binary as a AWS lambda function or simply run it on your machine.
### Workflow
<img src="https://s3.us-east-2.amazonaws.com/kepler-images/warrensbox/covid-vaccine-tracker/covid-vaccine-tracker-workflow-white-bg.svg" alt="drawing" style="width: 370px;"/>

1. CloudWatch will periodically trigger lambda.
1. The lambda function (Notifier app) will call the following API: ``
1. With the returned payload from the API, we will check against DynamoDB if the alert has been sent before. If it's the same as the previous alert, the function does nothing.
1. If the alert is new and is different than the previous alert, the function will trigger the SNS Topic.
1. All resources subscribing to the SNS topic will receive the alert.