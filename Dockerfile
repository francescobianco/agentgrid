FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o agentgrid .

FROM alpine:3.19
RUN apk add --no-cache docker-cli ca-certificates
WORKDIR /app
COPY --from=builder /app/agentgrid .
COPY --from=builder /app/templates ./templates
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./agentgrid"]
