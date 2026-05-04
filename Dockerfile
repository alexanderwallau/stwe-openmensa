FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go build -o stwe-openmensa .

FROM alpine:3.19

RUN adduser -D -u 1000 app
USER app

COPY --from=builder /app/stwe-openmensa /usr/local/bin/stwe-openmensa

EXPOSE 8080

ENTRYPOINT ["stwe-openmensa", "-listen", "0.0.0.0"]
