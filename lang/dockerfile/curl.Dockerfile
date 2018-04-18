FROM ubuntu
RUN apt-get update && apt-get install -y curl
CMD curl -I http://docker.com