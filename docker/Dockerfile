# syntax=docker/dockerfile:1

# ---- 1. build the React app ----
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# ---- 2. build the Go binaries (UI embedded) ----
# Pin a specific version (e.g. golang:1.25-alpine) for reproducible builds.
FROM golang:alpine AS build
ARG VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /web/dist ./internal/webui/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/natsradar ./cmd/natsradar \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/natsradar-seed ./cmd/natsradar-seed

# ---- 3. minimal runtime image ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/natsradar /natsradar
COPY --from=build /out/natsradar /natscope
COPY --from=build /out/natsradar-seed /natsradar-seed
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/natsradar"]
