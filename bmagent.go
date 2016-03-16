// Originally derived from: btcsuite/btcwallet/btcwallet.go
// Copyright (c) 2013-2014 The btcsuite developers

// Copyright (c) 2015 Monetas.
// Copyright 2016 Daniel Krawisz.
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"time"

	"github.com/DanielKrawisz/bmagent/rpc"
)

var (
	cfg             *config
	shutdownChannel = make(chan struct{})
)

func main() {
	// Use all processor cores.
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Work around defer not working after os.Exit.
	if err := bmclientMain(); err != nil {
		os.Exit(1)
	}
}

// bmclientMain is a work-around main function that is required since deferred
// functions (such as log flushing) are not called with calls to os.Exit.
// Instead, main runs this function and checks for a non-nil error, at which
// point any defers have already run, and if the error is non-nil, the program
// can be exited with an error exit status.
func bmclientMain() error {
	// Load configuration and parse command line.  This function also
	// initializes logging and configures it accordingly.
	tcfg, _, err := loadConfig()
	if err != nil {
		return err
	}
	cfg = tcfg
	defer backendLog.Flush()

	// Load the identities and message databases. The identities database must
	// have been created with the --create option already or this will return an
	// appropriate error.
	keymgr, store, err := openDatabases(cfg)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}
	defer store.Close()

	if cfg.Profile != "" {
		go func() {
			listenAddr := net.JoinHostPort("", cfg.Profile)
			log.Infof("Profile server listening on %s", listenAddr)
			profileRedirect := http.RedirectHandler("/debug/pprof",
				http.StatusSeeOther)
			http.Handle("/", profileRedirect)
			log.Errorf("%v", http.ListenAndServe(listenAddr, nil))
		}()
	}

	// Connect to bmd.
	rpcc, err := rpc.NewClient(&rpc.ClientConfig{
		DisableTLS: cfg.DisableClientTLS,
		CAFile:     cfg.CAFile,
		ConnectTo:  cfg.RPCConnect,
		Username:   cfg.BmdUsername,
		Password:   cfg.BmdPassword,
		Timeout:    time.Millisecond * 500, // TODO move to config
	})
	if err != nil {
		log.Errorf("Cannot create bmd server RPC client: %v", err)
		return err
	}

	// Initialize all servers.
	server, err := newServer(rpcc, keymgr, store)
	if err != nil {
		log.Errorf("Unable to create servers: %v", err)
		return err
	}

	// Start all servers.
	server.Start()

	// Shutdown the servers if an interrupt signal is received.
	addInterruptHandler(server.Stop)

	// Wait for the servers to shutdown either due to a stop RPC request
	// or an interrupt.
	server.WaitForShutdown()

	// Monitor for graceful server shutdown and signal the main goroutine
	// when done. This is done in a separate goroutine rather than waiting
	// directly so the main goroutine can be signaled for shutdown by either
	// a graceful shutdown or from the main interrupt handler. This is
	// necessary since the main goroutine must be kept running long enough
	// for the interrupt handler goroutine to finish.
	go func() {
		server.WaitForShutdown()
		log.Infof("Server shutdown complete")
		shutdownChannel <- struct{}{}
	}()

	// Wait for shutdown signal from either a graceful server stop or from
	// the interrupt handler.
	<-shutdownChannel
	log.Info("Shutdown complete")
	return nil
}