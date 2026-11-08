# Образ API-сервера и фонового обработчика (DV-014): multi-stage Go → distroless.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/kiryshabutor/SDLC/internal/platform.Version=${VERSION}" \
      -o /out/api ./cmd/api \
 && CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/kiryshabutor/SDLC/internal/platform.Version=${VERSION}" \
      -o /out/worker ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /out/worker /usr/local/bin/
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/api"]
