FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY worker/go.mod worker/go.sum ./
RUN go mod download

COPY worker/ ./

RUN CGO_ENABLED=0 GOOS=linux \
    go build -o /atlas-worker ./cmd

FROM alpine:3.22

WORKDIR /app

COPY --from=builder /atlas-worker /app/atlas-worker

CMD ["/app/atlas-worker"]