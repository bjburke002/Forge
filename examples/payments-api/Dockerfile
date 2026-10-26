FROM golang:1.27-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o main ./cmd/payments-api

FROM gcr.io/distroless/static:nonroot as runtime

WORKDIR /app

COPY --from=build /app/main .

EXPOSE 8080
EXPOSE 9090

ENTRYPOINT ["/app/main"]