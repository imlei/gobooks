# 遵循project_guide.md

FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a small static binary for Linux.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gobooks ./cmd/gobooks


FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/gobooks /app/gobooks
COPY internal/web/static /app/internal/web/static

ENV APP_ADDR=:6768

EXPOSE 6768

CMD ["/app/gobooks"]

