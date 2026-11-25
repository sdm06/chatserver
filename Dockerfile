# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/chatserver ./cmd/chatserver

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/chatserver /chatserver
EXPOSE 4000 8080
USER nonroot:nonroot
ENTRYPOINT ["/chatserver"]
