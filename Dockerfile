FROM golang:1.24-alpine

WORKDIR /app

COPY go.mod /

RUN go mod download

COPY . . 

RUN go build -o redis .

EXPOSE 6379

CMD ["/app/redis"]



