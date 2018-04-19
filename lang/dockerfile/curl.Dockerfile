FROM alpine
RUN apk add --update curl html2text
CMD curl -F "maxwidth=80" \
         -F "fontsize=8" \
         -F "webaddress=https://i1.wp.com/blog.docker.com/wp-content/uploads/2013/06/Docker-logo-011.png" \
         -F "negative=N" \
         http://www.glassgiant.com/ascii/ascii.php > /tmp/docker.html && \
    html2text /tmp/docker.html