ARG GO_VERSION=1.25
FROM golang:${GO_VERSION}-alpine as builder

WORKDIR /app

COPY go.mod .
RUN go mod download

COPY . .

ARG CHALLENGE_NAME=prime-time

RUN go build -o /app/main ./cmd/${CHALLENGE_NAME}

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/main .

EXPOSE 8080

ENTRYPOINT ["./main"]
