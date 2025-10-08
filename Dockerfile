FROM gcr.io/distroless/static-debian12
COPY app /app
ENTRYPOINT ["/app"]
