# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/todox .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app
WORKDIR /app

COPY --from=build /out/todox ./todox
COPY templates ./templates
COPY static ./static

RUN mkdir -p /data && chown -R app:app /data /app
USER app

ENV DB_PATH=/data/todos.db
ENV PORT=8080
EXPOSE 8080

ENTRYPOINT ["./todox"]
