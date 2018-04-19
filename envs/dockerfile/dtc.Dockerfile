FROM alpine
RUN apk add --update git python3 docker && pip3 install docker-compose
CMD git clone https://github.com/silvin-lubecki/docker-teaches-code.git /src/github.com/silvin-lubecki && \
    cd /src/github.com/silvin-lubecki && \
    docker-compose build && \
    docker run --rm -p 8081:8080 -v /var/run/docker.sock:/var/run/docker.sock -v /tmp/dtc:/tmp/dtc docker-teaches-code