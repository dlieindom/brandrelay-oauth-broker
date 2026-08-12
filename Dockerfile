FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/brandrelay-oauth-broker .
FROM alpine:3.20
RUN adduser -D -u 10001 app
USER app
COPY --from=build /out/brandrelay-oauth-broker /app/broker
EXPOSE 8080
ENTRYPOINT ["/app/broker"]
