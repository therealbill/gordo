package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/codegangsta/cli"
	"github.com/therealbill/libredis/client"
)

var port int
var auth RedisOption

func enslaveNode(slaveof RedisOption) {
	rc, err := client.DialWithConfig(&client.DialConfig{Address: fmt.Sprintf("127.0.0.1:%d", port), Password: auth.Value})
	if err == nil {
		d := strings.Split(slaveof.Value, " ")
		rc.SlaveOf(d[0], d[1])
	} else {
		log.Printf("Error on slaveof: '%v'", err)
	}
}

var (
	app *cli.App
)

func main() {
	app = cli.NewApp()
	app.Name = "gordo"
	app.Version = "0.1"
	app.EnableBashCompletion = true
	author := cli.Author{Name: "Bill Anderson", Email: "therealbill@me.com"}
	app.Authors = append(app.Authors, author)
	default_duration, _ := time.ParseDuration("60s")
	app.Flags = []cli.Flag{
		cli.DurationFlag{
			Name:   "enforceinterval,e",
			Usage:  "Duration to wait in between config enforcement passes",
			EnvVar: "GORDO_EINTERVAL",
			Value:  default_duration,
		},
		cli.BoolFlag{
			Name:   "waitforstart,w",
			Usage:  "Wait for the APi server to start",
			EnvVar: "GORDO_WAIT",
		},
		cli.BoolFlag{
			Name:   "fixedmem,f",
			Usage:  "Prevent changes to maxmemory",
			EnvVar: "GORDO_FIXEDMEM",
		},
		cli.BoolFlag{
			Name:   "automem,a",
			Usage:  "Autocalculate maxmemory",
			EnvVar: "GORDO_AUTOMEM",
		},
		cli.StringFlag{
			Name:   "maxmemory,m",
			Usage:  "Specify Redis' maxmemory",
			EnvVar: "GORDO_MAXMEM",
			Value:  "1G",
		},
		cli.StringFlag{
			Name:   "password,p",
			Usage:  "Specify Redis' password",
			EnvVar: "GORDO_RPASS",
			Value:  "secretpass",
		},
	}
	app.Action = serve
	app.Run(os.Args)
}

func serve(c *cli.Context) {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	var (
		mem  RedisOption
		auth RedisOption
	)

	if c.Bool("automem") {
		log.Printf("Calculating max memory...")
		// TODO: actually autocalculate maxmemory
		log.Printf("Done")
		mem = RedisOption{"maxmemory", c.String("maxmemory")}
	} else {
		mem = RedisOption{"maxmemory", c.String("maxmemory")}
	}

	//mem := RedisOption{"maxmemory", "4GB"}

	// this all needs reworked
	auth = RedisOption{"requirepass", c.String("password")}
	masterauth := RedisOption{"masterauth", c.String("password")}
	nologo := RedisOption{"syslog-enabled", "yes"}
	verbose := RedisOption{"loglevel", "debug"}
	daemon := RedisOption{"daemonize", "no"}
	myport := RedisOption{"port", "6380"}
	bind := RedisOption{"bind", "0.0.0.0"}
	//slaveof := RedisOption{"slaveof", "127.0.0.1 6379"}
	socket := RedisOption{"unixsocket", "/tmp/redis.sock"}
	dbfile := RedisOption{"dbfilename", "slave.rdb"}
	args := RedisArgs{"maxmemory": mem.Value,
		"requirepass":    auth.Value,
		"syslog-enabled": nologo.Value,
		"loglevel":       verbose.Value,
		"daemonize":      daemon.Value,
		"unixsocket":     socket.Value,
		"port":           myport.Value,
		"masterauth":     masterauth.Value,
		"dbfilename":     dbfile.Value,
		"bind":           bind.Value,
	}
	port, _ = strconv.Atoi(myport.Value)
	//enslaveNode(slaveof)

	apiserver := APIServer{}
	apiserver.Args = &args
	apiserver.FixedMemory = c.Bool("fixedmem")

	go func() {
		_ = <-sigs
		log.Print("Caught signal, closing Redis")
		//let us call a shutdown here
		rc, err := client.DialWithConfig(&client.DialConfig{Address: fmt.Sprintf("127.0.0.1:%d", port), Password: auth.Value})
		if err == nil {
			rc.Shutdown(true)
		} else {
			log.Printf("Error on shutdown: '%v'", err)
			log.Printf("Redis is already dead")
		}
		log.Print("Redis shutdown")
		done <- true
	}()

	as := args.Listify()
	log.Printf("Args: '%s'", as)
	binary := "redis-server"
	apiserver.RedisBinary = binary
	apiserver.DoneChan = done
	apiserver.EnforceInterval = c.Duration("enforceinterval")
	apiserver.Serve()
	//go runRedis(binary, done, args)
	<-done
}
