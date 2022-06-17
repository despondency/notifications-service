FROM golang:1.18.2-alpine AS builder

ENV PATH="/go/bin:${PATH}"
ENV GO111MODULE=on
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOARCH=amd64

WORKDIR $GOPATH/src/notifications-service/
COPY . .

RUN go mod download
RUN go mod verify

RUN apk -U add ca-certificates
RUN apk update && apk upgrade && apk add pkgconf git bash build-base sudo
RUN git clone https://github.com/edenhill/librdkafka.git && cd librdkafka && ./configure --prefix /usr && make && make install

COPY . .

RUN go build -tags musl -ldflags '-linkmode external -extldflags "-fno-PIC -static"' -o /go/bin/notificationsd ./cmd/main.go

FROM scratch AS runner

WORKDIR .
COPY --from=builder go/src/notifications-service/migrations ./migrations
COPY --from=builder /go/bin/notificationsd .

ENTRYPOINT ["./notificationsd"]