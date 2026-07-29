FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags="-s -w" -o /sshrpg ./src

FROM alpine:3.23
RUN addgroup -S game && adduser -S -G game game
WORKDIR /app
COPY --from=build /sshrpg /usr/local/bin/sshrpg
COPY --from=build /src/content /app/content
RUN mkdir /app/data && chown game:game /app/data
USER game
ENV SSH_LISTEN_ADDR=:2222
ENV SSH_HOST_KEY_PATH=/app/data/ssh_host_ed25519
ENV DATABASE_PATH=/app/data/game.db
ENV GAME_CONFIG_PATH=/app/content/game.json
ENV AREAS_PATH=/app/content/areas
ENV ITEMS_PATH=/app/content/items
ENV ENEMIES_PATH=/app/content/enemies
ENV QUESTS_PATH=/app/content/quests
EXPOSE 2222
VOLUME ["/app/data"]
ENTRYPOINT ["/usr/local/bin/sshrpg"]
