FROM golang:alpine

WORKDIR /go/src/github.com/PolarGeospatialCenter/pgcboot
COPY main.go ./main.go
COPY Gopkg.toml Gopkg.lock ./

RUN apk add --no-cache git make curl
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure
RUN go build -o /bin/qtainer .

FROM scratch
COPY --from=0 /bin/qtainer /bin/qtainer
CMD /bin/qtainer
