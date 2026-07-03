# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/leaderboard ./services/leaderboard-service/cmd/server

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/leaderboard /leaderboard
EXPOSE 8080
ENTRYPOINT ["/leaderboard"]
