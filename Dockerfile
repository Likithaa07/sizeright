# Stage 1: build (same Go version as go.mod)
FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /sizer ./cmd/sizer

# Stage 2: run (tiny image, no shell, non-root user)
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /sizer /sizer
COPY rules /rules
ENV RULES_PATH=/rules/v1.yaml
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/sizer"]
