package main

import (
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

func httpError(w http.ResponseWriter, err error) {
	logrus.Error(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func checkpoint(w http.ResponseWriter, r *http.Request) {
	logrus.Info("starting checkpoint")
	if err := exec.Command("runc", "checkpoint").Run(); err != nil {
		logrus.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logrus.Info("finished checkpoint successfully")
}

func restore(w http.ResponseWriter, r *http.Request) {
	logrus.Info("starting restore")
	done := make(chan error, 1)
	go func() {
		out, err := command("runc", "restore")
		if err != nil {
			logrus.Infof("%s", out)
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			logrus.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case <-time.After(1700 * time.Millisecond):
		logrus.Info("container restored successfully")
		return
	}
}

func run(w http.ResponseWriter, r *http.Request) {
	logrus.Info("starting inital run")
	done := make(chan error, 1)
	go func() {
		err := exec.Command("runc").Run()
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			logrus.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case <-time.After(1700 * time.Millisecond):
		logrus.Info("container started successfully")
		return
	}

}

func rsync(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ip := r.Form.Get("ip")
	if ip == "" {
		http.Error(w, "no ip specified", http.StatusBadRequest)
		return
	}
	if err := command("rsync", "-az", "--delete", "/root/ioquake3/q3a/", "root@"+ip+":/root/ioquake3/q3a"); err != nil {
		httpError(w, err)
		return
	}
	if err := command("rsync", "-az", "--delete", "/root/ioquake3/checkpoint/", "root@"+ip+":/root/ioquake3/checkpoint"); err != nil {
		httpError(w, err)
		return
	}
}

func command(p string, args ...string) error {
	out, err := exec.Command(p, args...).CombinedOutput()
	if err != nil {
		logrus.Infof("%s", out)
		return err
	}
	return nil
}

func main() {
	if err := os.Chdir("/root/ioquake3"); err != nil {
		logrus.Fatal(err)
	}
	addr := ":8080"
	h := mux.NewRouter()
	h.HandleFunc("/checkpoint", checkpoint).Methods("POST")
	h.HandleFunc("/restore", restore).Methods("POST")
	h.HandleFunc("/run", run).Methods("POST")
	h.HandleFunc("/rsync", rsync).Methods("POST")
	if err := http.ListenAndServe(addr, h); err != nil {
		logrus.Fatal(err)
	}
}
