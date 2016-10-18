FROM ubuntu:16.04
MAINTAINER Jason Lee <jawc@hotmail.com>

RUN apt-get update -y && apt-get install -y
RUN mkdir /jasonsoft/bifrost

ADD bifrost /jasonsoft/bifrost/bifrost

WORKDIR /jasonsoft/bifrost

CMD /jasonsoft/bifrost/bifrost