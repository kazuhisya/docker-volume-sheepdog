package main

import (
	"bufio"
	"errors"
	log "github.com/Sirupsen/logrus"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Check if the command is supported
func iscmdSupported(execCmd string) bool {
	_, err := exec.Command(execCmd, "-h").CombinedOutput()
	if err != nil {
		log.Debug("%s command not found on this host", execCmd)
		return false
	}
	return true
}

// dog vdi create volume 10G
func DogVdiCreate(vdiname, vdisize string) error {
	// Give the suffix for Docker Volume Plugin
	vdiname = "dvp-" + vdiname
	log.Debugf("Begin utils.DogVdiCreate: %s, %s", vdiname, vdisize)
	out, err := exec.Command("sudo", "dog", "vdi", "create", vdiname, vdisize, "-v").CombinedOutput()
	log.Debug("Result of DogVdiCreate: ", string(out))
	return err
}

// dog vdi delete volume
func DogVdiDelete(vdiname string) error {
	// Give the suffix for Docker Volume Plugin
	vdiname = "dvp-" + vdiname
	log.Debugf("Begin utils.DogVdiDelete: %s", vdiname)
	out, err := exec.Command("sudo", "dog", "vdi", "delete", vdiname).CombinedOutput()
	log.Debug("Result of DogVdiDelete: ", string(out))
	return err
}

// dog vdi list
func DogVdiList() (list string) {
	log.Debugf("Begin utils.DogVdiList:")
	cmd := "sudo dog vdi list -r |grep  ^= | grep dvp |cut -d' ' -f 2"
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to list vdi: ", err)
		return
	}
	list = string(out)
	return list
}

// dog vdi list (find)
func DogVdiExist(vdiname string) bool {
	// Give the suffix for Docker Volume Plugin
	vdiname = "dvp-" + vdiname
	log.Debugf("Begin utils.DogVdiExist: %s", vdiname)
	cmd := "sudo dog vdi list -r |grep  ^= | grep " + vdiname + "|cut -d' ' -f 2"
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to list vdi: ", err)
		return false
	}

	log.Debugf("out: %s", string(out))
	if string(out) != "" {
		log.Debugf("vdi exist")
		return true
	} else {
		log.Debugf("vdi not exist")
		return false
	}

}

// tgtadm --lld iscsi --mode target --op new --tid 1 --targetname iqn.2017-09.org.sheepdog-docker
func TgtTargetNew(tid, tname string) error {
	log.Debugf("Begin utils.TgtTargetNew: %s, %s", tid, tname)
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "target",
		"--op", "new", "--tid", tid, "--targetname", tname).CombinedOutput()
	log.Debug("Result of TgtTargetNew: ", string(out))
	return err
}

// tgtadm --lld iscsi --mode target --op bind --tid 1 --initiator-address 127.0.0.1
func TgtTargetBind(tid, tallow string) error {
	log.Debugf("Begin utils.TgtTargetBind: %s, %s", tid, tallow)
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "target",
		"--op", "bind", "--tid", tid, "--initiator-address", tallow).CombinedOutput()
	log.Debug("Result of TgtTargetBind: ", string(out))
	return err
}

// tgtadm --mode target --op show
// fixme

// tgtadm --lld iscsi --mode logicalunit --op new --tid 1 --lun 2 --bstype sheepdog --backing-store unix:/var/lib/sheepdog/sock:dvp-vol1
func TgtLunNew(tid, lun, vdiname string) error {
	// Give the suffix for Docker Volume Plugin
	vdiname = "dvp-" + vdiname
	log.Debugf("Begin utils.TgtLunNew: %s, %s, %s", tid, lun, vdiname)
	bstore := "unix:/var/lib/sheepdog/sock:" + vdiname
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "logicalunit",
		"--op", "new", "--tid", tid, "--lun", lun, "--bstype", "sheepdog",
		"--backing-store", bstore).CombinedOutput()
	log.Debug("Result of TgtLunNew: ", string(out))
	return err
}

// tgtadm --lld iscsi --mode logicalunit --op delete --tid 1 --lun 2
func TgtLunDelete(tid, lun string) error {
	log.Debugf("Begin utils.TgtLunDelete: %s, %s", tid, lun)
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "logicalunit",
		"--op", "delete", "--tid", tid, "--lun", lun).CombinedOutput()
	log.Debug("Result of TgtLunDelete: ", string(out))
	return err
}

// iscsiadm -m discovery -t st -p 127.0.0.1:3260
func iscsiDiscovery(tportal string) (targets []string, err error) {
	log.Debugf("Begin utils.iscsiDiscovery (portal: %s)", tportal)
	out, err := exec.Command("sudo", "iscsiadm", "--mode", "discovery",
		"--type", "sendtargets", "--portal", tportal).CombinedOutput()
	if err != nil {
		log.Error("Error encountered in sendtargets cmd: ", out)
		return
	}
	targets = strings.Split(string(out), "\n")
	return
}

// iscsiadm -m node -T iqn.2017-09.org.sheepdog-docker -l
func iscsiLogin(tiqn, tportal string) (err error) {
	log.Debugf("Begin utils.iscsiLogin: %s", tiqn)
	_, err = exec.Command("sudo", "iscsiadm", "--mode", "node",
		"--targetname", tiqn, "--portal", tportal, "--login").CombinedOutput()
	if err != nil {
		log.Errorf("Received error on login attempt: %v", err)
	}
	return err
}

// iscsiadm -m node -T iqn.2017-09.org.sheepdog-docker --portal 127.0.0.1:3260 -u
func iscsiDisableDelete(tiqn, tportal string) (err error) {
	log.Debugf("Begin utils.iscsiDisableDelete: %s", tiqn)
	_, err = exec.Command("sudo", "iscsiadm", "--mode", "node",
		"--targetname", tiqn, "--portal", tportal, "--logout").CombinedOutput()
	if err != nil {
		log.Debugf("Error during iscsi logout: ", err)
	}
	_, err = exec.Command("sudo", "iscsiadm", "--mode", "node",
		"--targetname", tiqn, "--op", "delete").CombinedOutput()
	return
}

// iscsiadm -m session --rescan
func iscsiRescan() bool {
	log.Debugf("Begin utils.iscsiRescan")
	out, err := exec.Command("sudo", "iscsiadm", "--mode", "session", "--rescan").CombinedOutput()
	if err != nil {
		log.Error("Error encountered in session rescan cmd: ", out)
		return false
	}
	return true
}

// GetDeviceNameFromLun
func GetDeviceNameFromLun(tip, tport, tipn, lun string) string {
	log.Debugf("Begin utils.GetDeviceNameFromLun: %s %s", tipn, lun)

	// "sleep hack" should change
	time.Sleep(3 * time.Second)

	path := "/dev/disk/by-path/ip-" + tip + ":" + tport + "-iscsi-" + tipn + "-lun-" + lun
	log.Debugf("path: %s", path)
	return path
}

// GetDeviceFileFromIscsiPath
func GetDeviceFileFromIscsiPath(iscsiPath string) (devFile string) {
	log.Debug("Begin utils.GetDeviceFileFromIscsiPath: ", iscsiPath)
	out, err := exec.Command("sudo", "ls", "-la", iscsiPath).CombinedOutput()
	if err != nil {
		log.Debug(err)
		return
	}
	d := strings.Split(string(out), "../../")
	log.Debugf("Found device: %s", d)
	devFile = "/dev/" + d[1]
	devFile = strings.TrimSpace(devFile)
	log.Debug("using base of: ", devFile)
	return
}

// GetFSType
func GetFSType(device string) string {
	log.Debugf("Begin utils.GetFSType: %s", device)
	fsType := ""
	out, err := exec.Command("blkid", device).CombinedOutput()
	if err != nil {
		return fsType
	}

	if strings.Contains(string(out), "TYPE=") {
		for _, v := range strings.Split(string(out), " ") {
			if strings.Contains(v, "TYPE=") {
				fsType = strings.Split(v, "=")[1]
				fsType = strings.Replace(fsType, "\"", "", -1)
			}
		}
	}
	return fsType
}

// FormatVolume
func FormatVolume(device, fsType string) error {
	log.Debugf("Begin utils.FormatVolume: %s, %s", device, fsType)
	cmd := "mkfs.ext4"
	if fsType == "xfs" {
		cmd = "mkfs.xfs"
	}
	log.Debug("Perform ", cmd, " on device: ", device)
	out, err := exec.Command(cmd, "-f", device).CombinedOutput()
	log.Debug("Result of mkfs cmd: ", string(out))

	return err
}

// Mount
func Mount(device, mountpoint string) error {
	log.Debugf("Begin utils.Mount device: %s on: %s", device, mountpoint)
	out, err := exec.Command("mkdir", "-p", mountpoint).CombinedOutput()
	out, err = exec.Command("mount", device, mountpoint).CombinedOutput()
	log.Debug("Response from mount ", device, " at ", mountpoint, ": ", string(out))
	if err != nil {
		log.Error("Error in mount: ", err)
	}
	return err
}

// Umount
func Umount(mountpoint string) error {
	log.Debugf("Begin utils.Umount: %s", mountpoint)
	out, err := exec.Command("umount", mountpoint).CombinedOutput()
	if err != nil {
		log.Warningf("Unmount call returned error: %s (%s)", err, out)
		if strings.Contains(string(out), "not mounted") {
			log.Debug("Ignore request for unmount on unmounted volume")
			err = errors.New("Volume is not mounted")
		}
	}
	return err
}

// find_vacant_lun
func FindVacantLun(tid string) (next_vacant_lun string) {
	var (
		fp         *os.File
		tgt_found  int
		curr_lun_i int
	)
	tgt_found = 0
	next_vacant_lun_int := 0

	log.Debugf("Begin utils.FindVacantLun")

	// tgtadm --mode target --op show
	out, err := exec.Command("sudo", "tgtadm", "--mode", "target", "--op", "show").CombinedOutput()
	if err != nil {
		log.Error("Failed to list contents of target options: ", err)
		return
	}
	content := []byte(out)
	ioutil.WriteFile("/tmp/target_list.txt", content, 0644)

	// The original implementation is here.
	// https://github.com/fujita/tgt/blob/master/scripts/tgt-setup-lun#L93-L113
	fp, err = os.Open("/tmp/target_list.txt")
	if err != nil {
		panic(err)
	}
	defer fp.Close()
	scanner := bufio.NewScanner(fp)

	for scanner.Scan() {
		// Check if we finished going over this target
		if (tgt_found == 1) && (strings.Contains(scanner.Text(), "Target") == true) {
			break
		}
		// Check if we found the requested target
		if strings.Contains(scanner.Text(), ("Target "+tid+":")) == true {
			tgt_found = 1
			continue
		}
		if (strings.Contains(scanner.Text(), "LUN:") == true) && (tgt_found == 1) {
			curr_lun := strings.Fields(scanner.Text())
			curr_lun_i, err = strconv.Atoi(curr_lun[1])
			if err != nil {
				panic(err)
			}
			if curr_lun_i > next_vacant_lun_int {
				break
			} else {
				next_vacant_lun_int = next_vacant_lun_int + 1
			}
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	next_vacant_lun = strconv.Itoa(next_vacant_lun_int)
	return next_vacant_lun
}
