FROM golang:1.18-alpine AS build
WORKDIR /app

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN go build .

FROM alpine:3.16.0
WORKDIR /app

RUN apk add tzdata
RUN adduser --disabled-password --no-create-home kaguya

COPY --from=build /app/kaguya .

USER kaguya
CMD ./kaguya
