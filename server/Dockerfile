# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cardwarsd ./cmd/cardwarsd

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cardwarsd /cardwarsd
COPY data /data
EXPOSE 8080
ENTRYPOINT ["/cardwarsd", "-addr", ":8080", "-data", "/data"]
