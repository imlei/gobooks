# 遵循project_guide.md

FROM golang:1.23-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build both the application binary and the migration runner.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gobooks         ./cmd/gobooks
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/gobooks-migrate ./cmd/gobooks-migrate


FROM alpine:3.20

WORKDIR /app

COPY --from=build /out/gobooks         /app/gobooks
COPY --from=build /out/gobooks-migrate /app/gobooks-migrate
COPY internal/web/static               /app/internal/web/static
# SQL migration files read at runtime by gobooks-migrate.
COPY migrations                        /app/migrations

ENV APP_ADDR=:6768

EXPOSE 6768

CMD ["/app/gobooks"]
