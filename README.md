# Gordo

Gordo is a Go based Redis process manager. It is intended to provide a
way to run Redis in docker when being used as part of a Sentinel managed
Redis pod or possibly a Redis Cluster cluster. As such it is a bit
opinionated.

## Status

This code is currently beta at best. Mostly proof of concept and working to
establish the basic pattern.

# The Problem

When running Redis in Docker as a standalone it works fine. However
because of how Sentinel works it can be broken if you do not pass the
`--net=host` option to your Docker container because Redis will see it's
Docker IP which is reachable only by the Docker host other containers on it.

# The Solution

The proper solution is for Redis to be able to have it's "announce
IP/Port" modified at runtime. The details on this can be found at
[http://bit.ly/1OUvXDI](My DevDay2015 Proposal). Until this is
implemented, however, we need a decent workaround. This is where Gordo
comes in.

# The Workaround

The workaround is to be able to spin up the container with Redis not yet
running and have a way to start it with the information needed. Gordo is
essentially a daemon which will listen for this information on a port
which you can then make a call to set this information and launch Redis.

The idea here is that because the IP/Port information for the container
is not normally available until after it starts we can look that up and
then start the Redis process. Ultimately, if a fix like the one I've
proposed is implemented Gordo will be able to handle that with no change
to the code configuring it.

# The API

Will go here once I've decided on it. First I want to get the process
management portion working.
