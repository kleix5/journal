FROM golang:1.25 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY web ./web

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/journal ./cmd/journal

FROM alpine:3.21

WORKDIR /app

COPY --from=build /out/journal /app/journal
COPY web /app/web

EXPOSE 8080

CMD ["/app/journal"]
