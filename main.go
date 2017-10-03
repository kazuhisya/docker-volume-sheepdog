package main

import (
	"errors"
	"flag"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/docker/go-plugins-helpers/volume"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

var (
	// Version release number
	Version = "0.1.2"
	// Short commit id from git
	// If this var isn't given when building, this value will be used.
	gitHash = "Custom Build"
	// buildDate from time.Now
	buildDate = string(time.Now().Format("2006-01-02"))
	// go version
	// e.g. "go1.8.3 linux/amd64"
	o         = runtime.GOOS
	a         = runtime.GOARCH
	v         = runtime.Version()
	goVersion = v + " " + o + "/" + a
)

var (
	defaultDir = filepath.Join(volume.DefaultDockerRootDirectory, "sheepdog")
	debug      = flag.Bool("debug", false, "Enable debug logging")
	version    = flag.Bool("version", false, "Version of Docker Volume Plugin for Sheepdog")
	cfgFile    = flag.String("config", "/etc/docker-volume-plugin.d/sheepdog.json", "path to config file")
)

func main() {
	flag.Parse()
	if *version {
		fmt.Println("Docker Volume Plugin for Sheepdog")
		fmt.Printf("Version : %s (%s)\n", Version, gitHash)
		fmt.Printf("Build at %s with %s\n", buildDate, goVersion)
		os.Exit(0)
	}
	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.Info("Starting sheepdog-docker-driver version: ", Version)

	// check cmd support
	if iscmdSupported("sudo") == false {
		err := errors.New("sudo command not found on this host")
		log.Error(err)
		os.Exit(1)
	}
	if iscmdSupported("iscsiadm") == false {
		err := errors.New("iscsi-initiator-utils(iscsiadm command) not found on this host")
		log.Error(err)
		os.Exit(1)
	}
	if iscmdSupported("tgtadm") == false {
		err := errors.New("scsi-target-utils(tgtadm command) not found on this host")
		log.Error(err)
		os.Exit(1)
	}
	if iscmdSupported("dog") == false {
		err := errors.New("sheepdog(dog command) not found on this host")
		log.Error(err)
		os.Exit(1)
	}

	u, _ := user.Lookup("root")
	gid, _ := strconv.Atoi(u.Gid)

	d := newSheepdogDriver(*cfgFile)
	h := volume.NewHandler(d)
	log.Info(h.ServeUnix("sheepdog", gid))
}
