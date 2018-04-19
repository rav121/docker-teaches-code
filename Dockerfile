FROM golang:1.10 AS gobuilder
WORKDIR /go/src/github.com/silvin-lubecki/docker-teaches-code
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /backend

FROM docker:dind
COPY front front
COPY lang lang
COPY --from=gobuilder /backend /

EXPOSE 8080
CMD /backend
