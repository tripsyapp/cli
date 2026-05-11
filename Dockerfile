FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=
ARG DATE=
RUN CGO_ENABLED=0 GOOS=linux go build \
	-ldflags "-s -w -X github.com/tripsyapp/cli/internal/cli.Version=${VERSION} -X github.com/tripsyapp/cli/internal/cli.Commit=${COMMIT} -X github.com/tripsyapp/cli/internal/cli.Date=${DATE}" \
	-o /tripsy-mcp ./cmd/tripsy-mcp

FROM alpine:3.22

RUN addgroup -S tripsy && adduser -S -G tripsy tripsy

COPY --from=build /tripsy-mcp /usr/local/bin/tripsy-mcp

USER tripsy

ENV PORT=8080
EXPOSE 8080

CMD ["sh", "-c", "tripsy-mcp --transport http --http-addr 0.0.0.0:${PORT} --http-path /mcp --disable-raw-request"]
