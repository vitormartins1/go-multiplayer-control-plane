# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/leaderboard ./services/leaderboard-service/cmd/server
# Worker do fan-out durável (ADR-0020): mesmo image, entrypoint escolhido no compose.
RUN CGO_ENABLED=0 go build -o /bin/leaderboard-consumer ./services/leaderboard-service/cmd/consumer

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/leaderboard /leaderboard
COPY --from=build /bin/leaderboard-consumer /leaderboard-consumer
EXPOSE 8080
ENTRYPOINT ["/leaderboard"]
