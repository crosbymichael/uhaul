package main

import (
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
)

var activeCmd *exec.Cmd

func httpError(w http.ResponseWriter, err error) {
	logrus.Error(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func checkpoint(w http.ResponseWriter, r *http.Request) {
	logrus.Warn("[+] Checkpoint container")
	if activeCmd == nil {
		logrus.Warnf("Checkpoint called without any active container")
	}

	logrus.Info("Executing: runc checkpoint")
	if err := exec.Command("runc", "checkpoint").Run(); err != nil {
		logrus.Error(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	logrus.Info("Container checkpointed successfully")
	activeCmd = nil
}

func restore(w http.ResponseWriter, r *http.Request) {
	logrus.Warn("[+] Restoring container")
	if activeCmd != nil {
		logrus.Warnf("Restore called with an active container")
	}

	cmd := exec.Command("runc", "restore")
	done := make(chan error, 1)
	go func() {
		logrus.Info("Executing: runc checkpoint")
		done <- cmd.Run()
	}()
	select {
	case err := <-done:
		if err != nil {
			logrus.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case <-time.After(1700 * time.Millisecond):
		logrus.Info("Container restored successfully")
		activeCmd = cmd
		return
	}
}

func run(w http.ResponseWriter, r *http.Request) {
	logrus.Warn("[+] Starting new container")
	if activeCmd != nil {
		logrus.Warnf("Run called with an active container")
	}

	cmd := exec.Command("runc")
	done := make(chan error, 1)
	go func() {
		logrus.Info("Executing: runc")
		done <- cmd.Run()
	}()
	select {
	case err := <-done:
		if err != nil {
			logrus.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	case <-time.After(1700 * time.Millisecond):
		logrus.Info("Container started successfully")
		activeCmd = cmd
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
	logrus.Info("Transfering checkpoint data...")
	if err := exec.Command("rsync", "-az", "--delete", "/root/ioquake3/checkpoint", "/root/ioquake3/q3a", "root@"+ip+":/root/ioquake3/").Run(); err != nil {
		httpError(w, err)
		return
	}
}

func reset(w http.ResponseWriter, r *http.Request) {
	logrus.Warnf("[+] Resetting node (active command = %t)", activeCmd != nil)
	if activeCmd != nil {
		logrus.Info("Killing active process")
		if err := activeCmd.Process.Kill(); err != nil {
			logrus.Warnf("Error killing active process: %v", err)
		}
		activeCmd = nil
	}
	logrus.Info("Removing container state")
	os.RemoveAll("/var/run/ocf/ioquake3/")
}

func main() {
	if err := os.Chdir("/root/ioquake3"); err != nil {
		logrus.Fatal(err)
	}
	addr := ":8080"
	h := mux.NewRouter()
	h.HandleFunc("/checkpoint", checkpoint).Methods("POST")
	h.HandleFunc("/reset", reset).Methods("POST")
	h.HandleFunc("/restore", restore).Methods("POST")
	h.HandleFunc("/run", run).Methods("POST")
	h.HandleFunc("/rsync", rsync).Methods("POST")
	logrus.Warn("[*] Starting node")
	if err := http.ListenAndServe(addr, h); err != nil {
		logrus.Fatal(err)
	}
}
