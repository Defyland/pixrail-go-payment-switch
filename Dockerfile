FROM golang:1.25.11-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/pixrail-api ./cmd/pixrail-api \
  && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/pixrail-worker ./cmd/pixrail-worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/pixrail-api /pixrail-api
COPY --from=build /out/pixrail-worker /pixrail-worker
EXPOSE 8080
ENTRYPOINT ["/pixrail-api"]
