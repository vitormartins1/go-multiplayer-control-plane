# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.25-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/identity ./services/identity-service/cmd/server

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/identity /identity
EXPOSE 8080
ENTRYPOINT ["/identity"]
