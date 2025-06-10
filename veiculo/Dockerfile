FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o veiculo .

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/veiculo ./veiculo
ENTRYPOINT ["./veiculo"] 