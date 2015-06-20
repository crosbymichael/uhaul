package main

import (
	"encoding/json"
	"net/http"
	"os/exec"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

var localAddr = "45.55.31.66"

type server struct {
	Active bool   `json:"active"`
	IP     string `json:"ip"`
}

func (s *server) checkpoint() error {
	if err := s.do("POST", "/checkpoint"); err != nil {
		return err
	}
	s.Active = false
	return nil
}

func (s *server) do(method, url string) error {
	r, err := http.NewRequest(method, "http://"+s.IP+":8080"+url, nil)
	if err != nil {
		return err
	}
	if _, err = http.DefaultClient.Do(r); err != nil {
		return err
	}
	return nil
}

func (s *server) rsync(to *server) error {
	return s.do("POST", "/rsync?ip="+to.IP)
}

func (s *server) run() error {
	if err := s.do("POST", "/run"); err != nil {
		return err
	}
	s.Active = true
	return nil
}

func (s *server) restore() error {
	if err := s.do("POST", "/restore"); err != nil {
		return err
	}
	s.Active = true
	return nil
}

func (s *server) reset() error {
	if err := s.do("POST", "/reset"); err != nil {
		return err
	}
	s.Active = false
	return nil
}

func configureNetwork(to *server) error {
	rules := [][]string{
		{
			"OUTPUT", "--dst", localAddr, "-p", "udp", "--dport", "27960", "-j", "DNAT", "--to-destination", to.IP + ":27960",
		},
		{
			"POSTROUTING", "-p", "udp", "--dst", to.IP, "--dport", "27960", "-j", "SNAT", "--to-source", localAddr,
		},
		{
			"PREROUTING", "-p", "udp", "-d", localAddr, "--dport", "27960", "-j", "DNAT", "--to-destination", to.IP + ":27960",
		},
	}
	if err := flushAll(); err != nil {
		return err
	}
	for _, rule := range rules {
		if err := command("iptables", append([]string{"-t", "nat", "-A"}, rule...)...); err != nil {
			return err
		}
	}
	command("conntrack", "-D", "-p", "udp")
	return nil
}

func flushAll() error {
	if err := command("iptables", "-t", "nat", "-F"); err != nil {
		logrus.Error(err)
	}
	return nil
}

func command(p string, args ...string) error {
	out, err := exec.Command(p, args...).CombinedOutput()
	if err != nil {
		logrus.Infof("%s", out)
		return err
	}
	return nil
}

type servers []*server

func (ss servers) active() *server {
	for _, s := range ss {
		if s.Active {
			return s
		}
	}
	return nil
}

func (ss servers) get(id string) *server {
	for _, s := range ss {
		if s.IP == id {
			return s
		}
	}
	return nil
}

var activeServers servers

func list(w http.ResponseWriter, r *http.Request) {
	if err := json.NewEncoder(w).Encode(activeServers); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func reset(w http.ResponseWriter, r *http.Request) {
	for _, server := range activeServers {
		server.reset()
	}
}

func start(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ip := r.Form.Get("ip")
	s := activeServers.get(ip)
	if s == nil {
		s = &server{
			IP: ip,
		}
		activeServers = append(activeServers, s)
	}
	if s.Active {
		return
	}
	activeServer := activeServers.active()
	if activeServer == nil {
		if err := s.run(); err != nil {
			httpError(w, err)
			return
		}
		if err := configureNetwork(s); err != nil {
			httpError(w, err)
		}
		return
	}
	if err := activeServer.checkpoint(); err != nil {
		httpError(w, err)
		return
	}
	if err := activeServer.rsync(s); err != nil {
		httpError(w, err)
		return
	}
	if err := configureNetwork(s); err != nil {
		httpError(w, err)
		return
	}
	if err := s.restore(); err != nil {
		httpError(w, err)
		return
	}
}

func httpError(w http.ResponseWriter, err error) {
	logrus.Error(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func main() {
	flushAll()
	addr := ":8080"
	h := mux.NewRouter()
	h.HandleFunc("/", list).Methods("GET")
	h.HandleFunc("/reset", reset).Methods("POST")
	h.HandleFunc("/start", start).Methods("POST")
	if err := http.ListenAndServe(addr, h); err != nil {
		logrus.Fatal(err)
	}
}
