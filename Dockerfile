# syntax=docker/dockerfile:1

## Build
FROM golang:buster AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN go build -o /cardtable-scrape-decks

## Deploy
FROM gcr.io/distroless/base-debian10

WORKDIR /

COPY --from=build /cardtable-scrape-decks /cardtable-scrape-decks

EXPOSE 8281

USER nonroot:nonroot

ENTRYPOINT ["/cardtable-scrape-decks"]

