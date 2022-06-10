FROM golang:1.18-alpine AS builder

WORKDIR /app
COPY . .

RUN go build -o ./main main.go

FROM alpine:3.16 AS runner

WORKDIR /app
COPY secrets.json .
COPY env.json .
COPY channel-ids.json .
COPY --from=builder /app/main .

ENTRYPOINT ./main