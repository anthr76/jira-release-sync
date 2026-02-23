FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /jira-release-sync ./cmd/jira-release-sync

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /jira-release-sync /jira-release-sync
ENTRYPOINT ["/jira-release-sync"]
