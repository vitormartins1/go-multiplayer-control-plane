# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/achievements ./services/achievements-service/cmd/server
# Worker do fan-out durável (ADR-0020): mesmo image, entrypoint escolhido no compose.
RUN CGO_ENABLED=0 go build -o /bin/achievements-consumer ./services/achievements-service/cmd/consumer

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/achievements /achievements
COPY --from=build /bin/achievements-consumer /achievements-consumer
EXPOSE 8080
ENTRYPOINT ["/achievements"]
