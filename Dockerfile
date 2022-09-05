###################################
# 1. Build in a Go-based image   #
###################################
FROM golang:1.19-alpine as builder
RUN apk add --no-cache git # add deps here (like make) if needed
WORKDIR /go/cubby
COPY . .
# any pre-requisities to building should be added here
# RUN go generate
RUN go build -v

###################################
# 2. Copy into a clean image     #
###################################
FROM alpine:latest
COPY --from=builder /go/cubby/cubby /cubby
VOLUME /data
# expose port if needed
EXPOSE 8080
ENTRYPOINT ["/cubby"]
# any flags here, for example use the data folder
CMD ["serve", "-port", "8080", "-path","/data/cubby.db"]

