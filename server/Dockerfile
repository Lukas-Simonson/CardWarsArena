# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/cardwarsd ./cmd/cardwarsd
RUN mkdir /out/state

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/cardwarsd /cardwarsd
COPY data /data
# Where the database goes (-db): owned by the nonroot user, so a volume mounted here is writable.
COPY --from=build --chown=65532:65532 /out/state /var/lib/cardwars
EXPOSE 8080
ENTRYPOINT ["/cardwarsd", "-addr", ":8080", "-data", "/data"]
