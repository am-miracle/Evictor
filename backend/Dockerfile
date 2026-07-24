FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /evictor ./cmd/evictor

FROM alpine:3.22
RUN addgroup -S evictor && adduser -S -G evictor evictor
USER evictor
COPY --from=build /evictor /usr/local/bin/evictor
EXPOSE 8080
ENTRYPOINT ["evictor"]
