FROM golang:1.19-alpine as build
WORKDIR /
COPY . .
RUN go mod download
RUN go build -o node ./app/p2p_snow_node/main.go

FROM alpine:latest
WORKDIR /
COPY --from=build /node ./node
ENTRYPOINT ["./node"]