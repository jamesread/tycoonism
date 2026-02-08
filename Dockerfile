FROM --platform=linux/amd64 alpine:3.20

LABEL org.opencontainers.image.title=webfarm

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY server /app/server
COPY frontend/dist /app/dist
COPY service/data /app/data

EXPOSE 4838

ENTRYPOINT ["/app/server"]
