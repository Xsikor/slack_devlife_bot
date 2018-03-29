FROM golang:alpine

ADD . /app
WORKDIR /app

RUN go build -o /app/slackBot main.go

#SlackChanel for messages. @nickName or #channel
ENV SLACK_CHAN @a.malik
ENV SLACK_WEBHOOK_URL http://localhost.com/slack/webhook/url

CMD /app/slackBot
