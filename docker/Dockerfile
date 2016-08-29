FROM ubuntu:16.04
MAINTAINER Jason Lee <jawc@hotmail.com>

RUN apt-get update -y && apt-get install -y
RUN mkdir /opt/bifrost

ADD bifrost /opt/bifrost/bifrost

WORKDIR /opt/bifrost

CMD /opt/bifrost/bifrost