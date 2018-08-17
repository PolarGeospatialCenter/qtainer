FROM golang:stretch

WORKDIR /go/src/github.com/PolarGeospatialCenter/pgcboot
COPY main.go ./main.go
COPY Gopkg.toml Gopkg.lock ./

RUN apt-get install -y git make curl
RUN curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
RUN dep ensure -v -vendor-only
RUN go build -o /bin/qtainer .

FROM scratch
COPY --from=0 /bin/qtainer /bin/qtainer
CMD /bin/qtainer
