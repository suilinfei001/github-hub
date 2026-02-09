FROM alpine:3.19
RUN addgroup -S app && adduser -S -G app app
COPY bin/ghh-server /usr/local/bin/ghh-server
COPY bin/ghh /usr/local/bin/ghh
WORKDIR /app
RUN mkdir -p /data && chown -R app:app /data
USER app
EXPOSE 8080
VOLUME ["/data"]
ENV GITHUB_TOKEN=""
ENTRYPOINT ["/usr/local/bin/ghh-server"]
CMD ["--addr", ":8080", "--root", "/data"]
