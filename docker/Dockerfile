# syntax=docker/dockerfile:1

# ---- 1. build the React app ----
FROM node:26-alpine AS web
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
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/natscope ./cmd/natscope \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/natscope-seed ./cmd/natscope-seed

# ---- 3. minimal runtime image ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/natscope /natscope
COPY --from=build /out/natscope-seed /natscope-seed
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/natscope"]
