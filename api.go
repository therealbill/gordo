package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/therealbill/libredis/client"
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"
)

type Response struct {
	Message string
	Status  string
	Data    interface{}
}

type OptionMessage struct {
	Option  string
	Value   string
	Restart bool
}

type DirectiveMessage struct {
	Directive string
	Value     string
}

type APIServer struct {
	Args            *RedisArgs
	RedisBinary     string
	DoneChan        chan bool
	FixedMemory     bool
	EnforceInterval time.Duration
}

// throwJSONParseError is used when the JSON a client submits via the API isn't
// parseable
func throwJSONParseError(req *http.Request) (retcode int, userMessage string) {
	retcode = 422
	userMessage = "JSON Parse failure"
	em := fmt.Errorf(userMessage)
	log.Print(em)
	//e := airbrake.ExtendedNotification{ErrorClass: "Request.ParseJSON", Error: em}
	//err := airbrake.ExtendedError(e, req)
	//if err != nil {
	//log.Print("airbrake error:", err)
	//}
	return
}

func (s *APIServer) getClient() (*client.Redis, error) {
	password, _ := s.Args.GetOption("requirepass")
	sock, _ := s.Args.GetOption("unixsocket")
	log.Printf("connecting to %s with %s", sock, password)
	rc, err := client.DialWithConfig(&client.DialConfig{Address: sock, Password: password, Network: "unix"})
	if err != nil {
		return rc, err
	}
	rc.ClientSetName("gordo-control")
	return rc, err
}

func (s *APIServer) getDirective(c web.C, w http.ResponseWriter, r *http.Request) {
	var response Response
	target := c.URLParams["directive"]
	log.Printf("getDirective called: '%v'", target)
	if target == "maxmemory" {
		response.Data, _ = s.Args.GetOption("maxmemory")
		response.Status = "success"
		resp, _ := json.Marshal(response)
		w.Write(resp)
		return
	}
	rc, err := s.getClient()
	if err != nil {
		response = Response{Status: "connection error", Message: err.Error()}
	} else {
		val, err := rc.ConfigGet(target)
		if err != nil {
			log.Printf("Error on config get call: '%v", err)
		} else {
			response.Data = val
			response.Status = "success"
			s.Args.SetOption(target, val[target])
		}
	}
	resp, _ := json.Marshal(response)
	w.Write(resp)
}

func (s *APIServer) setOption(c web.C, w http.ResponseWriter, r *http.Request) {
	log.Print("setOption called")
	target := c.URLParams["option"]
	var (
		response Response
		reqdata  OptionMessage
	)
	body, err := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(body, &reqdata)
	if err != nil {
		retcode, em := throwJSONParseError(r)
		log.Print(em)
		http.Error(w, em, retcode)
	}
	switch reqdata.Option {
	case "maxmemory":
		if s.FixedMemory {
			response.Message = "Altering Redis' Maximum memory not allowed"
			response.Status = "prohibited"
		} else {
			s.Args.SetOption("maxmemory", reqdata.Value)
			response.Message = "changed"
			response.Status = "success"
		}
	default:
		s.Args.SetOption(target, reqdata.Value)
		response.Message = "changed"
		response.Status = "success"
	}
	if reqdata.Restart {
		s.restartRedis(c, w, r)
	} else {
		resp, _ := json.Marshal(response)
		w.Write(resp)
	}
}

func (s *APIServer) setDirective(c web.C, w http.ResponseWriter, r *http.Request) {
	log.Print("setDirective called")
	var (
		response Response
		reqdata  DirectiveMessage
	)
	body, err := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(body, &reqdata)
	if err != nil {
		retcode, em := throwJSONParseError(r)
		log.Print(em)
		http.Error(w, em, retcode)
	}
	switch reqdata.Directive {
	case "maxmemory":
		if s.FixedMemory {
			response.Status = "prohibited"
			response.Message = "Altering maxmemory not allowed"
		} else {
			rc, err := s.getClient()
			if err != nil {
				response = Response{Status: "connection error", Message: err.Error()}
			} else {
				err := rc.ConfigSet(reqdata.Directive, reqdata.Value)
				if err != nil {
					log.Printf("Error on config set call: '%v", err)
				} else {
					response.Status = "success"
					s.Args.SetOption(reqdata.Directive, reqdata.Value)
				}
			}
		}
	default:
		rc, err := s.getClient()
		if err != nil {
			response = Response{Status: "connection error", Message: err.Error()}
		} else {
			err := rc.ConfigSet(reqdata.Directive, reqdata.Value)
			if err != nil {
				log.Printf("Error on config set call: '%v", err)
			} else {
				response.Status = "success"
				s.Args.SetOption(reqdata.Directive, reqdata.Value)
			}
		}
	}
	resp, _ := json.Marshal(response)
	w.Write(resp)
}

func (s *APIServer) getConfig(c web.C, w http.ResponseWriter, r *http.Request) {
	log.Printf("getConfig called")
	resp, _ := json.Marshal(s.Args)
	w.Write(resp)
}

func (s *APIServer) getOption(c web.C, w http.ResponseWriter, r *http.Request) {
	target := c.URLParams["directive"]
	d, _ := s.Args.GetOption(target)
	//resp, _ := json.Marshal(d)
	//w.Write(resp)
	w.Write([]byte(d))
}

func (s *APIServer) startRedis(c web.C, w http.ResponseWriter, r *http.Request) {
	log.Printf("Starting Redis via API")
	go runRedis(s.RedisBinary, s.DoneChan, s.Args)
	go s.enforceOptions()
}

func (s *APIServer) restartRedis(c web.C, w http.ResponseWriter, r *http.Request) {
	s.stopRedis(c, w, r)
	s.startRedis(c, w, r)
}

func (s *APIServer) stopRedis(c web.C, w http.ResponseWriter, r *http.Request) {
	var response []byte
	rc, err := s.getClient()
	if err == nil {
		rc.Shutdown(true)
	} else {
		log.Printf("Error on shutdown: '%v'", err)
		log.Printf("Redis is already dead")
	}
	log.Print("Redis shutdown")
	response = []byte("shutdown complete")
	w.Write(response)
}

func (s *APIServer) enforceOptions() {
	for {
		rc, err := s.getClient()
		if err != nil {
			log.Printf("Redis connection failed: %v", err)
		} else {
			maxmem, _ := s.Args.GetOption("maxmemory")
			log.Printf("Enforcing Memory Setting: %s", maxmem)
			e := rc.ConfigSet("maxmemory", maxmem)
			if e != nil {
				log.Printf("Error enforcing memory setting on local Redis: '%v'", e)
			}
			log.Printf("Sleeping for %v", s.EnforceInterval)
		}
		time.Sleep(s.EnforceInterval)
	}
}

func (s *APIServer) Serve() {
	goji.Get("/config/all", s.getConfig)
	goji.Get("/config/:directive", s.getDirective)
	goji.Put("/config/:directive", s.setDirective)
	goji.Put("/option/:option", s.setOption)
	goji.Get("/restart", s.restartRedis)
	goji.Get("/start", s.startRedis)
	goji.Get("/stop", s.stopRedis)
	goji.Serve()
}
