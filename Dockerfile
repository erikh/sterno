FROM golang AS builder
COPY . /build
WORKDIR /build
RUN go build -o /sterno .

FROM debian

COPY sterno.conf /etc
COPY --from=builder /sterno /bin
CMD /bin/sterno -config /etc/sterno.conf
