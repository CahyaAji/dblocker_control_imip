FROM golang:1.25-alpine AS builder

WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /app/server ./cmd

FROM node:22-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./
RUN npm install

COPY frontend/ ./
RUN npm run build

FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /app/server /app/server
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

EXPOSE 8080
ENTRYPOINT ["/app/server"]
