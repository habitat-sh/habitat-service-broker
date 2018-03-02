FROM busybox

ADD servicebroker /opt/servicebroker/servicebroker
COPY items/nginx-0.1.0.tgz /opt/servicebroker/items/nginx-0.1.0.tgz
COPY items/redis-1.1.13.tgz /opt/servicebroker/items/redis-1.1.13.tgz

CMD /opt/servicebroker/servicebroker -alsologtostderr
