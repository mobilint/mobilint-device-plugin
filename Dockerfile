FROM golang:1.26.1 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -buildvcs=false -o /mobilint-device-plugin ./cmd/mobilint-device-plugin

FROM gcr.io/distroless/static-debian12

COPY --from=build /mobilint-device-plugin /mobilint-device-plugin
ENTRYPOINT ["/mobilint-device-plugin"]
