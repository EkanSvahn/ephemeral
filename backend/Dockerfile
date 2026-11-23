FROM golang:1.27.4-alpine AS build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# No GOARCH: the binary is built for the architecture the image is built
# for, so the same Dockerfile works on amd64 and arm64 hosts.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api
EXPOSE 8080
ENTRYPOINT ["/api"]
