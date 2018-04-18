FROM docker:dind
VOLUME [ "/dtc" ]
CMD docker build -f /dtc/Dockerfile -t tmp-dtc-docker /dtc && \
    docker run --rm -v /var/run/docker.sock:/var/run/docker.sock tmp-dtc-docker
