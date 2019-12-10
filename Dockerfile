FROM golang:1.13

WORKDIR /go/src/github.com/filabrazilska/rekki-test
COPY . .

RUN go get -d -v ./...
RUN go install -v ./...

CMD ["rekki-test"]
