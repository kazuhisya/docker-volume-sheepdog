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
	"strconv"
)

const (
	// VERSION release number
	VERSION = "0.0.1"
)

var (
	defaultDir = filepath.Join(volume.DefaultDockerRootDirectory, "sheepdog")
	debug      = flag.Bool("debug", false, "Enable debug logging")
	version    = flag.Bool("version", false, "Version of Docker Volume Plugin for Sheepdog")
	cfgFile    = flag.String("config", "/etc/sheepdog/dockerdriver.json", "path to config file")
)

func main() {
	flag.Parse()
	if *version {
		fmt.Println("Docker Volume Plugin for Sheepdog")
		fmt.Println("Version : ", VERSION)
		os.Exit(0)
	}
	if *debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.Info("Starting sheepdog-docker-driver version: ", VERSION)

	// check cmd support
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
