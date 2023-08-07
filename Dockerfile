FROM docker.io/golang:1.19 as builder

WORKDIR /build
COPY go.mod go.sum *.go ./
RUN go get -d .
RUN CGO_ENABLED=0 GOOS=linux go build -a -o grab .

FROM alpine:latest

WORKDIR /

COPY --from=builder /build/grab ./
ENTRYPOINT ["./grab"]
