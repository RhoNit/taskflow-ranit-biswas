FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /taskflow      ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /taskflow-migrate ./cmd/migrate

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /taskflow          /usr/local/bin/taskflow
COPY --from=builder /taskflow-migrate  /usr/local/bin/taskflow-migrate

EXPOSE 8066

ENTRYPOINT ["taskflow"]
