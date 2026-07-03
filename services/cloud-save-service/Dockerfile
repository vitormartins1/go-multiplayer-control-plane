# Build (contexto = raiz do monorepo; módulo único)
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/cloud-save ./services/cloud-save-service/cmd/server

# Runtime
FROM gcr.io/distroless/static-debian12
COPY --from=build /bin/cloud-save /cloud-save
EXPOSE 8080
ENTRYPOINT ["/cloud-save"]
