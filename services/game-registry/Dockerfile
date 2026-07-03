# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/game-registry ./services/game-registry/cmd/server

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/game-registry /game-registry
EXPOSE 8080
ENTRYPOINT ["/game-registry"]
