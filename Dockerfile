FROM golang:1.10 AS gobuilder
COPY . .
RUN go build -o backend .
EXPOSE 8080
CMD ["./backend"]
