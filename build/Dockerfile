FROM golang:1.20.3-alpine as builder

ENV CGO_ENABLED=0
WORKDIR /build
COPY . .
RUN go build -o /build/bin/app .


FROM scratch

WORKDIR /server
COPY --from=builder /build/bin/app .

ENTRYPOINT ["./app"]
