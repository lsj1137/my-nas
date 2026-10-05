FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go test ./... && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /nas-gateway .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /nas-gateway /nas-gateway
USER nonroot:nonroot
EXPOSE 8086
ENTRYPOINT ["/nas-gateway"]
