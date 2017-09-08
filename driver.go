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

type Config struct {
	DefaultVolSz   string
	MountPoint     string
	TargetId       string
	TargetIqn      string
	TargetBindIp   string
	TargetBindPort string
}

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

	if conf.TargetId == "" {
		conf.TargetId = "1"
	}
	if conf.TargetIqn == "" {
		conf.TargetIqn = "iqn.2017-09.org.sheepdog-docker"
	}
	if conf.TargetBindIp == "" {
		conf.TargetBindIp = "127.0.0.1"
	}
	if conf.TargetBindPort == "" {
		conf.TargetBindPort = "3260"
	}

	log.Infof("Using config file: %s", cfg)
	log.Infof("Set MountPoint to: %s", conf.MountPoint)
	log.Infof("Set DefaultVolSz to: %s", conf.DefaultVolSz)

	log.Infof("Set TargetId to: %s", conf.TargetId)
	log.Infof("Set TargetIqn to: %s", conf.TargetIqn)
	log.Infof("Set TargetBindIp to: %s", conf.TargetBindIp)
	log.Infof("Set TargetBindPort to: %s", conf.TargetBindPort)

	return conf, nil
}

func prepareTarget(tid string, tiqn string, tip string, tport string) bool {
	log.Infof("Start TgtTargetNew")
	err := TgtTargetNew(tid, tiqn)
	if err != nil {
		log.Debug("Error unit.TgtTargetNew: ", err)
	}

	log.Infof("Start TgtTargetBind")
	err = TgtTargetBind(tid, tip)
	if err != nil {
		log.Debug("Error unit.TgtTargetBind: ", err)
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
		log.Fatal("Error processing cinder driver config file: ", err)
	}

	targetid := conf.TargetId
	targetiqn := conf.TargetIqn
	targetbindip := conf.TargetBindIp
	targetbindport := conf.TargetBindPort
	prepareTarget(targetid, targetiqn, targetbindip, targetbindport)

	_, err = os.Lstat(conf.MountPoint)
	if os.IsNotExist(err) {
		if err := os.MkdirAll(conf.MountPoint, 0755); err != nil {
			log.Fatal("Failed to create Mount directory during driver init: %v", err)
		}
	}

	d := SheepdogDriver{
		Conf:  &conf,
		Mutex: &sync.Mutex{},
	}

	return d
}

func (d SheepdogDriver) Create(r volume.Request) volume.Response {
	log.Infof("Create: %s, %v", r.Name, r.Options)
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	err := DogVdiCreate(r.Name, d.Conf.DefaultVolSz)
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

func (d SheepdogDriver) Remove(r volume.Request) volume.Response {
	log.Infof("Remove: %s", r.Name)

	err := DogVdiDelete(r.Name)
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

func (d SheepdogDriver) Path(r volume.Request) volume.Response {
	log.Infof("Path: %s", r.Name)
	path := filepath.Join(d.Conf.MountPoint, r.Name)
	log.Debug("Path reported as: ", path)
	return volume.Response{Mountpoint: path}
}

func (d SheepdogDriver) Mount(r volume.MountRequest) volume.Response {
	log.Infof("Mount: %s", r.Name)
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	// target new
	log.Debug("create new lun")
	lun := FindVacantLun(d.Conf.TargetId)
	log.Debug("lun: %s", lun)
	err := TgtLunNew(d.Conf.TargetId, lun, r.Name)
	if err != nil {
		log.Fatal("Error create new lun: ", err)
	}

	// iscsiadm -m session --rescan
	log.Debug("sescan session")
	iscsiRescan()

	// mapping disk
	device := GetDeviceNameFromLun(d.Conf.TargetBindIp, d.Conf.TargetBindPort, d.Conf.TargetIqn, lun)
	realdevice := strings.TrimSpace(GetDeviceFileFromIscsiPath(device))
	log.Debug("realdevice: %s", realdevice)

	// mkfs
	if GetFSType(realdevice) == "" {
		log.Debugf("Formatting device")
		err := FormatVolume(realdevice, "xfs")
		if err != nil {
			err := errors.New("Failed to format device")
			log.Error(err)
			return volume.Response{Err: err.Error()}
		}
	}

	// mount
	if mountErr := Mount(realdevice, d.Conf.MountPoint+"/"+r.Name); mountErr != nil {
		err := errors.New("Problem mounting docker volume ")
		log.Error(err)
		return volume.Response{Err: err.Error()}
	}

	return volume.Response{Mountpoint: d.Conf.MountPoint + "/" + r.Name}
}

func (d SheepdogDriver) Unmount(r volume.UnmountRequest) volume.Response {
	log.Infof("Unmount: %s", r.Name)
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	if umountErr := Umount(d.Conf.MountPoint + "/" + r.Name); umountErr != nil {
		if umountErr.Error() == "Volume is not mounted" {
			log.Warning("Request to unmount volume, but it's not mounted")
			return volume.Response{}
		} else {
			return volume.Response{Err: umountErr.Error()}
		}
	}

	err := iscsiDisableDelete(d.Conf.TargetIqn, string(d.Conf.TargetBindIp+":"+d.Conf.TargetBindPort))
	if err != nil {
		log.Debug("Error unit.iscsiLogin: ", err)
	}

	iscsiRescan()
	return volume.Response{}
}

func (d SheepdogDriver) Get(r volume.Request) volume.Response {
	log.Infof("Get: %s", r.Name)
	path := filepath.Join(d.Conf.MountPoint, r.Name)
	log.Infof("Get path: %s", path)

	vdiexist := DogVdiExist(r.Name)
	if vdiexist == true {
		return volume.Response{Volume: &volume.Volume{Name: r.Name, Mountpoint: path}}
	} else {
		log.Debugf("Failed to retrieve volume named: ", r.Name)
		err := errors.New("Volume Not Found")
		return volume.Response{Err: err.Error()}
	}

}

func (d SheepdogDriver) List(r volume.Request) volume.Response {
	log.Info("List volumes:")
	d.Mutex.Lock()
	defer d.Mutex.Unlock()

	path := filepath.Join(d.Conf.MountPoint, r.Name)
	var vols []*volume.Volume
	if vols != nil {
		return volume.Response{}
	}

	out := DogVdiList()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "dvp") {
			volname := strings.Replace(line, "dvp-", "", -1)
			vol := &volume.Volume{Name: volname, Mountpoint: (path + "/" + volname)}
			vols = append(vols, vol)
			log.Debug("vol: %s", vol)
		}
	}

	return volume.Response{Volumes: vols}
}

func (d SheepdogDriver) Capabilities(r volume.Request) volume.Response {
	var res volume.Response
	res.Capabilities = volume.Capability{Scope: "global"}
	return res
}
