FROM golang:1.10 AS gobuilder
COPY *.go .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /backend

FROM docker:dind
COPY front front
COPY lang lang
COPY --from=gobuilder /backend /

EXPOSE 8080
CMD /backend
