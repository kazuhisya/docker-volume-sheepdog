package main

import (
	"encoding/json"
	"errors"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/go-plugins-helpers/volume"
)

// Config model
type Config struct {
	DefaultVolSz   string
	MountPoint     string
	TargetID       string
	TargetIqn      string
	TargetBindIP   string
	TargetBindPort string
	VdiSuffix      string
	mountCount     map[string]int
}

// SheepdogDriver model
type SheepdogDriver struct {
	Mutex *sync.Mutex
	Conf  *Config
}

func processConfig(cfg string) (Config, error) {
	var conf Config
	content, err := ioutil.ReadFile(cfg)
	if err != nil {
		log.Fatal("Error reading config file: ", err)
	}
	err = json.Unmarshal(content, &conf)
	if err != nil {
		log.Fatal("Error parsing json config file: ", err)
	}

	if conf.MountPoint == "" {
		conf.MountPoint = "/mnt/sheepdog/mount"
	}
	if conf.DefaultVolSz == "" {
		conf.DefaultVolSz = "10G"
	}

	if conf.TargetID == "" {
		conf.TargetID = "1"
	}
	if conf.TargetIqn == "" {
		conf.TargetIqn = "iqn.2017-09.org.sheepdog-docker"
	}
	if conf.TargetBindIP == "" {
		conf.TargetBindIP = "127.0.0.1"
	}
	if conf.TargetBindPort == "" {
		conf.TargetBindPort = "3260"
	}
	if conf.VdiSuffix == "" {
		conf.VdiSuffix = "dvp"
	}

	// Max 128 Lun, include lun 0 ?
	conf.mountCount = make(map[string]int, 127)

	log.Infof("Using config file: %s", cfg)
	log.Infof("Set MountPoint to: %s", conf.MountPoint)
	log.Infof("Set DefaultVolSz to: %s", conf.DefaultVolSz)

	log.Infof("Set TargetID to: %s", conf.TargetID)
	log.Infof("Set TargetIqn to: %s", conf.TargetIqn)
	log.Infof("Set TargetBindIP to: %s", conf.TargetBindIP)
	log.Infof("Set TargetBindPort to: %s", conf.TargetBindPort)

	log.Infof("Set VdiSuffix to: %s", conf.VdiSuffix)

	return conf, nil
}

func prepareTarget(tid string, tiqn string, tip string, tport string) bool {
	log.Infof("Start tgtTargetNew")
	err := tgtTargetNew(tid, tiqn)
	if err != nil {
		log.Debug("Error unit.tgtTargetNew: ", err)
	}

	log.Infof("Start tgtTargetBind")
	err = tgtTargetBind(tid, tip)
	if err != nil {
		log.Debug("Error unit.tgtTargetBind: ", err)
	}

	log.Infof("Start iscsiDiscovery")
	targets, err := iscsiDiscovery(string(tip + ":" + tport))
	if err != nil {
		log.Debug("Error unit.iscsiDiscovery: ", err)
	}
	log.Debug("Discovery target: %w", targets)

	log.Infof("Start iscsiLogin")
	err = iscsiLogin(tiqn, string(tip+":"+tport))
	if err != nil {
		log.Debug("Error unit.iscsiLogin: ", err)
	}
	// fixme: Actually, that haven't checked anything yet. it should be improvement.
	return true
}

func newSheepdogDriver(cfgFile string) SheepdogDriver {
	conf, err := processConfig(cfgFile)
	if err != nil {
		log.Fatal("Error processing sheepdog driver config file: ", err)
	}

	targetid := conf.TargetID
	targetiqn := conf.TargetIqn
	targetbindip := conf.TargetBindIP
	targetbindport := conf.TargetBindPort
	prepareTarget(targetid, targetiqn, targetbindip, targetbindport)

	_, err = os.Lstat(conf.MountPoint)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(conf.MountPoint, 0755); err != nil {
			log.Errorf("Failed to create Mount directory during driver init: %v", err)
		}
	}

	d := SheepdogDriver{
		Conf:  &conf,
		Mutex: &sync.Mutex{},
	}

	return d
}

// Create API
func (d SheepdogDriver) Create(r volume.Request) volume.Response {
	log.Infof("Create: %s, %v", r.Name, r.Options)
	var volumeSize string
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	// Handle options (unrecognized options are silently ignored):
	// size: If there is no explicit designation, use the value of
	// config or default setting.
	if optsSize, ok := r.Options["size"]; ok {
		volumeSize = optsSize
	} else {
		// Assume the default root
		volumeSize = d.Conf.DefaultVolSz
	}

	vdiname := d.Conf.VdiSuffix + "-" + r.Name
	err := dogVdiCreate(vdiname, volumeSize)
	if err != nil {
		err := errors.New("Failed to create vdi")
		log.Error(err)
		return volume.Response{Err: err.Error()}
	}

	path := filepath.Join(d.Conf.MountPoint, r.Name)
	if err := os.Mkdir(path, 0755); err != nil {
		log.Errorf("Failed to create Mount directory: %v", err)
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{}
}

// Remove API
func (d SheepdogDriver) Remove(r volume.Request) volume.Response {
	log.Infof("Remove: %s", r.Name)

	log.Debug("Count %s", d.Conf.mountCount[r.Name])
	if d.Conf.mountCount[r.Name] != 0 {
		err := errors.New("This volume is currently used by other container")
		log.Error(err)
		return volume.Response{Err: err.Error()}
	}
	delete(d.Conf.mountCount, r.Name)

	vdiname := d.Conf.VdiSuffix + "-" + r.Name
	err := dogVdiDelete(vdiname)
	if err != nil {
		err := errors.New("Failed to delete vdi")
		log.Error(err)
		return volume.Response{Err: err.Error()}
	}

	path := filepath.Join(d.Conf.MountPoint, r.Name)
	log.Debug("remove path: ", path)
	if err := os.Remove(path); err != nil {
		log.Errorf("Failed to remove Mount directory: %v", err)
		return volume.Response{Err: err.Error()}
	}
	return volume.Response{}
}

// Path API
func (d SheepdogDriver) Path(r volume.Request) volume.Response {
	log.Infof("Path: %s", r.Name)
	path := filepath.Join(d.Conf.MountPoint, r.Name)
	log.Debug("Path reported as: ", path)
	return volume.Response{Mountpoint: path}
}

// Mount API
func (d SheepdogDriver) Mount(r volume.MountRequest) volume.Response {
	log.Infof("Mount: %s", r.Name)
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	// make sure that it is already mounting for another container
	if isAlreadyMountingThisVolume(d.Conf.MountPoint+"/"+r.Name) == true {
		// already mounting
		log.Infof("Mountpoint is already used: %s", r.Name)
		d.Conf.mountCount[r.Name]++
		log.Debug("Count %s", d.Conf.mountCount[r.Name])
		// skip all and return now
		return volume.Response{Mountpoint: d.Conf.MountPoint + "/" + r.Name}
	}
	// double check
	log.Debug("Count %s", d.Conf.mountCount[r.Name])
	if d.Conf.mountCount[r.Name] != 0 {
		log.Infof("Mountpoint is already used: %s", r.Name)
		d.Conf.mountCount[r.Name]++
		log.Debug("Count %s", d.Conf.mountCount[r.Name])
		return volume.Response{Mountpoint: d.Conf.MountPoint + "/" + r.Name}
	}

	// target new
	log.Debug("create new lun")
	lun := findVacantLun(d.Conf.TargetID)
	log.Debug("lun: %s", lun)
	vdiname := d.Conf.VdiSuffix + "-" + r.Name
	err := tgtLunNew(d.Conf.TargetID, lun, vdiname)
	if err != nil {
		log.Fatal("Error create new lun: ", err)
	}

	// iscsiadm -m session --rescan
	log.Debug("rescan session")
	iscsiRescan()

	// mapping disk
	device := getDeviceNameFromLun(d.Conf.TargetBindIP, d.Conf.TargetBindPort, d.Conf.TargetIqn, lun)
	realdevice := strings.TrimSpace(getDeviceFileFromIscsiPath(device))
	log.Debug("realdevice: %s", realdevice)

	// mkfs
	if getFSType(realdevice) == "" {
		log.Debugf("Formatting device")
		err := formatVolume(realdevice, "xfs")
		if err != nil {
			err := errors.New("Failed to format device")
			log.Error(err)
			return volume.Response{Err: err.Error()}
		}
	}

	// mount
	if mountErr := mount(realdevice, d.Conf.MountPoint+"/"+r.Name); mountErr != nil {
		err := errors.New("Problem mounting docker volume ")
		log.Error(err)
		return volume.Response{Err: err.Error()}
	}

	log.Debug("Count %s", d.Conf.mountCount[r.Name])
	d.Conf.mountCount[r.Name]++
	log.Debug("Count %s", d.Conf.mountCount[r.Name])

	return volume.Response{Mountpoint: d.Conf.MountPoint + "/" + r.Name}
}

// Unmount API
func (d SheepdogDriver) Unmount(r volume.UnmountRequest) volume.Response {
	log.Infof("Unmount: %s", r.Name)
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	log.Debug("Count %s", d.Conf.mountCount[r.Name])
	d.Conf.mountCount[r.Name]--
	log.Debug("Count %s", d.Conf.mountCount[r.Name])

	lun := getLunFromDeviceName(r.Name)
	scsi := getScsiNameFromDeviceName(r.Name)

	if d.Conf.mountCount[r.Name] <= 0 {
		if umountErr := umount(d.Conf.MountPoint + "/" + r.Name); umountErr != nil {
			if umountErr.Error() == "Volume is not mounted" {
				log.Warning("Request to unmount volume, but it's not mounted")
				return volume.Response{}
			}
			return volume.Response{Err: umountErr.Error()}
		}

		err := iscsiDeleteDevice(scsi)
		if err != nil {
			log.Debug("Error unit.iscsiDeleteDevice: ", err)
		}

		err = tgtLunDelete(d.Conf.TargetID, lun)
		if err != nil {
			log.Debug("Error unit.tgtLunDelete: ", err)
		}

		iscsiRescan()

		log.Debug("Count %s", d.Conf.mountCount[r.Name])
		d.Conf.mountCount[r.Name] = 0
		log.Debug("Count %s", d.Conf.mountCount[r.Name])

		path := filepath.Join(d.Conf.MountPoint, r.Name)
		log.Debug("remove path: ", path)
		if err := os.Remove(path); err != nil {
			log.Errorf("Failed to remove Mount directory: %v", err)
			return volume.Response{Err: err.Error()}
		}
	}
	return volume.Response{}
}

// Get API
func (d SheepdogDriver) Get(r volume.Request) volume.Response {
	log.Infof("Get: %s", r.Name)
	path := filepath.Join(d.Conf.MountPoint, r.Name)
	log.Infof("Get path: %s", path)

	vdiname := d.Conf.VdiSuffix + "-" + r.Name
	vdiexist := dogVdiExist(vdiname)
	if vdiexist == true {
		return volume.Response{Volume: &volume.Volume{Name: r.Name, Mountpoint: path}}
	}

	log.Debugf("Failed to retrieve volume named: ", r.Name)
	err := errors.New("Volume Not Found")
	return volume.Response{Err: err.Error()}
}

// List API
func (d SheepdogDriver) List(r volume.Request) volume.Response {
	log.Info("List volumes:")
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	path := filepath.Join(d.Conf.MountPoint, r.Name)
	var vols []*volume.Volume
	if vols != nil {
		return volume.Response{}
	}

	out := dogVdiList(d.Conf.VdiSuffix)
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, d.Conf.VdiSuffix) {
			searchname := d.Conf.VdiSuffix + "-"
			volname := strings.Replace(line, searchname, "", -1)
			vol := &volume.Volume{Name: volname, Mountpoint: (path + "/" + volname)}
			vols = append(vols, vol)
			log.Debug("vol: %s", vol)
		}
	}

	return volume.Response{Volumes: vols}
}

// Capabilities API
func (d SheepdogDriver) Capabilities(r volume.Request) volume.Response {
	var res volume.Response
	res.Capabilities = volume.Capability{Scope: "global"}
	return res
}
