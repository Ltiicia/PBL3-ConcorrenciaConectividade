FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o veiculo main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/veiculo ./veiculo
ENTRYPOINT ["./veiculo"] 