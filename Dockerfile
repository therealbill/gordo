# sshd on archlinux
#
# VERSION               0.0.1
 
FROM     base/archlinux:latest
MAINTAINER 	Bill Anderson <bill.anderson@rackspace.com>

ADD redis-server /usr/bin/redis-server
ADD gordo-linux /gordo
 
# Run daemon
CMD ["/gordo"]
