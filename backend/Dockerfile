FROM golang:1.25 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /audit-backend .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /audit-backend /audit-backend
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/audit-backend"]
