FROM golang:1.22-alpine AS build
WORKDIR /src
COPY app/ .
RUN go mod tidy && CGO_ENABLED=0 go build -trimpath -o /out/meridian .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata curl && adduser -D -u 1001 mcapp
WORKDIR /app
COPY --from=build /out/meridian /app/meridian
COPY app/static /app/static
COPY app/data /app/data
COPY app/exports /app/exports
RUN mkdir -p /app/uploads /app/logs && cp /app/data/logs/app.log /app/logs/app.log \
    && chown -R mcapp:mcapp /app/uploads /app/logs
USER mcapp
EXPOSE 8080
CMD ["/app/meridian"]
