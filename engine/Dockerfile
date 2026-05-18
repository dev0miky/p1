FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG BINARY=api
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/app ./cmd/${BINARY}

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 10001 app
USER app
WORKDIR /app
COPY --from=build /out/app /app/app
EXPOSE 8080
ENTRYPOINT ["/app/app"]
