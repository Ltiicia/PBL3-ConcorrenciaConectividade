FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o empresa main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/empresa ./empresa
COPY ./data ./data
ENTRYPOINT ["./empresa"] 