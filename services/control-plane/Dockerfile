# syntax=docker/dockerfile:1
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/control-plane ./services/control-plane/cmd/control-plane

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/control-plane /control-plane
EXPOSE 8090
USER nonroot:nonroot
ENTRYPOINT ["/control-plane"]
