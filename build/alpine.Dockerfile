FROM golang:alpine as builder

ENV CGO_ENABLED=0
WORKDIR /build
COPY . .
RUN go build -o /build/bin/app .


FROM alpine

WORKDIR /server
COPY --from=builder /build/bin/app .

ENTRYPOINT ["./app"]
