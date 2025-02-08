FROM golang:latest AS builder
RUN mkdir -p /go/src/pipeline
WORKDIR /go/src/pipeline
ADD main.go .
ADD go.mod .
RUN go build -o pipeline main.go


FROM alpine:latest
LABEL version="1.0.0"
LABEL maintainer="Egor Chukavin <klin0312@gmail.com>"
WORKDIR /root/
COPY --from=builder /go/src/pipeline/pipeline .
ENTRYPOINT ./pipeline  
EXPOSE 8080
