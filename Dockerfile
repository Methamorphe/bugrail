# syntax=docker/dockerfile:1

FROM golang:1.23-bookworm AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -o /out/bugrail ./cmd/bugrail

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

ENV BUGRAIL_DATA_DIR=/data

VOLUME ["/data"]

COPY --from=build /out/bugrail /usr/local/bin/bugrail

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/usr/local/bin/bugrail"]
CMD ["serve"]
