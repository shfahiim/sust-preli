FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN adduser -D appuser
USER appuser
WORKDIR /app
COPY --from=build /out/server /app/server
ENV PORT=8000
EXPOSE 8000
CMD ["/app/server"]
