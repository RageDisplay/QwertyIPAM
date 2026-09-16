FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/vectrixipam .

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
RUN adduser -D -u 10001 vectrix
WORKDIR /app
COPY --from=build /out/vectrixipam .
RUN mkdir -p /data && chown -R vectrix:vectrix /data
USER vectrix
ENV VECTRIXIPAM_DB_PATH=/data/vectrixipam.db
ENV VECTRIXIPAM_LISTEN_ADDR=:8080
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/vectrixipam"]
