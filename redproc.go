package main

import (
	"bufio"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func processRedisLogMessage(entry string) {
	var logpairs []string
	if strings.Contains(entry, " - ") {
		logpairs = strings.Split(entry, " - ")
	} else if strings.Contains(entry, " * ") {
		logpairs = strings.Split(entry, " * ")
	}
	if len(logpairs) == 0 {
		return
	}
	meta := strings.Split(logpairs[0], " ")
	msg := logpairs[1]
	msgdata := strings.Split(msg, " ")
	if strings.Contains(msg, "clients connected") {
		client_count, _ := strconv.Atoi(msgdata[0])
		log.Printf("[METRICS] clientcount=%d", client_count)
		slave_count, err := strconv.Atoi(msgdata[3][1:])
		if err != nil {
			log.Print("Nope, didn't work")
		}
		log.Printf("[METRICS] slavecount=%d", slave_count)
	} else if msg == "Background saving terminated with success" {
		log.Printf("[EVENT] BGSAVE completed")
	} else if strings.HasPrefix(msg, "The server is now ready to accept ") {
		switch msgdata[8] {
		case "at":
			log.Printf("[EVENT] Redis Listening on UNIX socket at %s", msgdata[9])
		case "on":
			log.Printf("[EVENT] Redis Listening on port %s", msgdata[10])
		}
	} else if strings.HasSuffix(msg, "ready to start.") {
		msg = strings.TrimSuffix(msg, " ready to start.")
		version := msgdata[1]
		bitdepth, _ := strconv.Atoi(msgdata[3])
		subdata := strings.Split(msg, ", ")
		mode := strings.TrimSuffix(subdata[1], "mode")
		sport := strings.TrimPrefix(subdata[2], "port ")
		port, _ = strconv.Atoi(sport)
		pid, _ := strconv.Atoi(strings.TrimPrefix(subdata[3], "pid "))
		log.Printf("[META] version=%s", version)
		log.Printf("[META] bitdepth=%d", bitdepth)
		log.Printf("[META] mode=%s", mode)
		log.Printf("[META] port=%d", port)
		log.Printf("[META] pid=%d", pid)
	} else if strings.Contains(msg, "Accepted") {
		client := ""
		if msgdata[1] == "connection" {
			client = msgdata[3]
		} else {
			client = msgdata[1]
		}
		log.Printf("[EVENT] Connection: %s", client)
	} else if strings.HasPrefix(msg, "Increased maximum number of open files to ") {
		nrof, _ := strconv.Atoi(msgdata[7])
		orig, _ := strconv.Atoi(strings.TrimSuffix(msgdata[13], ")."))
		log.Printf("[EVENT] adjusted NROF from %d to %d", orig, nrof)
	} else if msg == "Client closed connection" {
		log.Print("[EVENT] Client Disconnection")
	} else if strings.HasSuffix(msg, "asks for synchronization") {
		slave := msgdata[1]
		log.Printf("[EVENT] %s requests sync", slave)
	} else if strings.HasPrefix(msg, "Full resync requested ") {
		slave := msgdata[5]
		log.Printf("[EVENT] %s requests FULL sync", slave)
	} else if strings.HasPrefix(msg, "Background saving started") {
		pid := msgdata[len(msgdata)-1]
		log.Printf("[EVENT] BGSAVE START by pid %s", pid)
	} else if strings.HasPrefix(msg, "Starting BGSAVE") {
		reason := msgdata[3]
		target := msgdata[6]
		log.Printf("[EVENT] BGSAVE to %s for %s", target, reason)
	} else if strings.HasPrefix(msg, "Synchronization with slave ") {
		slave := msgdata[3]
		status := msgdata[4]
		log.Printf("[STATUS] Slave %s sync: %s", slave, status)
	} else if strings.HasPrefix(msg, "DB ") {
		if strings.Contains(msg, "keys") {
			db := strings.TrimSuffix(msgdata[1], ":")
			keycount, _ := strconv.Atoi(msgdata[2])
			volatile, _ := strconv.Atoi(msgdata[4][1:])
			slots, _ := strconv.Atoi(msgdata[7])
			log.Printf("[METRIC] DB=%s keys=%d volatile=%d slots=%d", db, keycount, volatile, slots)
		} else if strings.HasPrefix(msg, "DB saved ") {
			target := msgdata[3]
			log.Printf("[EVENT] Persistence to %s completed", target)
		} else if strings.HasPrefix(msg, "DB loaded from disk:") {
			loadtime := msgdata[4]
			log.Printf("[METRIC] loadtime=%ss", loadtime)
		} else {
			log.Printf("msgdata: '%v'", msgdata)
		}
	} else if strings.HasPrefix(msg, "Saving the final") {
		log.Printf("[EVENT] Initiating SAVE for exit")
	} else if msg == "Removing the unix socket file." {
		log.Printf("[EVENT] %s", msg)

	} else if strings.HasPrefix(msg, "SLAVE OF") {
		// Now for slave lines
		master := msgdata[2]
		log.Printf("[EVENT] Enslaving to %s", master)
	} else if strings.HasPrefix(msg, "Connecting to MASTER") {
		master := msgdata[3]
		log.Printf("[EVENT] Connecting to %s", master)
	} else if msg == "MASTER <-> SLAVE sync started" {
		log.Printf("[EVENT] %s", msg)
	} else if strings.HasPrefix(msg, "Partial resynchronization not possible") {
		if strings.Contains(msg, "no cached master") {
			log.Print("[EVENT] PSYNC not possible, no cached master")
		} else {
			log.Printf("[MESSAGE] %s", msg)
		}
	} else if strings.HasPrefix(msg, "Full resync from master") {
		log.Printf("[MESSAGE] %s", msg)
	} else if strings.HasPrefix(msg, "MASTER <-> SLAVE sync:") {
		smsg := strings.TrimPrefix(msg, "MASTER <-> SLAVE sync:")
		log.Printf("[STATE] %s", smsg)
	} else if strings.HasPrefix(msg, "Master replied to PING") {
		log.Print("[EVENT] Master available")
	} else {
		fmt.Printf("redis stdout | %s\n", entry)
	}

	_ = meta

}

func runRedis(binary string, done chan bool, args *RedisArgs) {
	rargs := args.Listify()
	log.Printf("rargs: %+v", rargs)
	rcmd := exec.Command(binary, rargs...)
	cmdOut, _ := rcmd.StdoutPipe()
	scanner := bufio.NewScanner(cmdOut)
	go func() {
		for scanner.Scan() {
			processRedisLogMessage(scanner.Text())
		}
	}()
	rcmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	rcmd.Run()
	done <- true

}
