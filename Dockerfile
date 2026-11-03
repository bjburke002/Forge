FROM golang:1.27-alpine AS build

ARG TARGETOS TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -ldflags="-w -w" -o /out/forge-controller ./cmd/forge-controller

FROM gcr.io/distroless/static:nonroot AS runtime

WORKDIR /app

COPY --from=build /out/forge-controller .

EXPOSE 8080 8081

USER 65532

ENTRYPOINT ["/app/forge-controller"]