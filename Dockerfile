# syntax=docker/dockerfile:1
FROM golang:latest

WORKDIR /app

ADD . /app/

RUN go build .

EXPOSE 8080

ENTRYPOINT [""]