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
COPY --from=build /src/maps /app/maps
COPY --from=build /src/items /app/items
COPY --from=build /src/enemies /app/enemies
RUN mkdir /app/data && chown game:game /app/data
USER game
ENV SSH_LISTEN_ADDR=:2222
ENV SSH_HOST_KEY_PATH=/app/data/ssh_host_ed25519
ENV DATABASE_PATH=/app/data/game.db
ENV MAPS_PATH=/app/maps
ENV ITEMS_PATH=/app/items/items.json
ENV ENEMIES_PATH=/app/enemies/enemies.json
EXPOSE 2222
VOLUME ["/app/data"]
ENTRYPOINT ["/usr/local/bin/sshrpg"]
