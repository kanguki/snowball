FROM golang:1.19-alpine as build
WORKDIR /
COPY . .
RUN go mod download
RUN go build -o operator ./app/operator/main.go

FROM alpine:latest
WORKDIR /
COPY --from=build /operator ./operator
ENTRYPOINT ["./operator"]