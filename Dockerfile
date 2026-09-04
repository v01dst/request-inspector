FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
RUN go build -ldflags="-s -w" -o /inspector .

FROM alpine:3.20
COPY --from=build /inspector /usr/local/bin/inspector
ENV PORT=3000
EXPOSE 3000
ENTRYPOINT ["inspector"]
