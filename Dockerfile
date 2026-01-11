FROM golang:1.24 as builder
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/upstream-blog

FROM alpine:3
RUN apk add --no-cache ca-certificates
COPY --from=builder /bin/upstream-blog upstream-blog
COPY config.json config.json
COPY docs docs
CMD ["/upstream-blog"]
