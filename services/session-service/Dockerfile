# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/session ./services/session-service/cmd/server

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/session /session
EXPOSE 8080 50051
ENTRYPOINT ["/session"]
