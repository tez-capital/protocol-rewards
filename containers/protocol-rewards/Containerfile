FROM golang:1.22 as build

WORKDIR /go/src/app
COPY . .

RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o /go/bin/app

FROM gcr.io/distroless/static-debian12
COPY --from=build /go/bin/app /
CMD ["/app"]