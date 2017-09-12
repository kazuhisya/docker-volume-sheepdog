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
func dogVdiCreate(vdiname, vdisize string) error {
	log.Debugf("Begin utils.dogVdiCreate: %s, %s", vdiname, vdisize)
	out, err := exec.Command("sudo", "dog", "vdi", "create", vdiname, vdisize, "-v").CombinedOutput()
	log.Debug("Result of dogVdiCreate: ", string(out))
	return err
}

// dog vdi delete volume
func dogVdiDelete(vdiname string) error {
	log.Debugf("Begin utils.dogVdiDelete: %s", vdiname)
	out, err := exec.Command("sudo", "dog", "vdi", "delete", vdiname).CombinedOutput()
	log.Debug("Result of dogVdiDelete: ", string(out))
	return err
}

// dog vdi list
func dogVdiList(suffix string) (list string) {
	log.Debugf("Begin utils.dogVdiList:")
	cmd := "sudo dog vdi list -r |grep  ^= | grep " + suffix + " |cut -d' ' -f 2"
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to list vdi: ", err)
		return
	}
	list = string(out)
	return list
}

// dog vdi list (find)
func dogVdiExist(vdiname string) bool {
	log.Debugf("Begin utils.dogVdiExist: %s", vdiname)
	cmd := "sudo dog vdi list -r |grep  ^= | grep -w " + vdiname + "|cut -d' ' -f 2"
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to list vdi: ", err)
		return false
	}

	log.Debugf("out: %s", string(out))
	if string(out) != "" {
		log.Debugf("vdi exist")
		return true
	}
	log.Debugf("vdi not exist")
	return false
}

// tgtadm --lld iscsi --mode target --op new --tid 1 --targetname iqn.2017-09.org.sheepdog-docker
func tgtTargetNew(tid, tname string) error {
	log.Debugf("Begin utils.tgtTargetNew: %s, %s", tid, tname)
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "target",
		"--op", "new", "--tid", tid, "--targetname", tname).CombinedOutput()
	log.Debug("Result of tgtTargetNew: ", string(out))
	return err
}

// tgtadm --lld iscsi --mode target --op bind --tid 1 --initiator-address 127.0.0.1
func tgtTargetBind(tid, tallow string) error {
	log.Debugf("Begin utils.tgtTargetBind: %s, %s", tid, tallow)
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "target",
		"--op", "bind", "--tid", tid, "--initiator-address", tallow).CombinedOutput()
	log.Debug("Result of tgtTargetBind: ", string(out))
	return err
}

// tgtadm --lld iscsi --mode logicalunit --op new --tid 1 --lun 2 --bstype sheepdog --backing-store unix:/var/lib/sheepdog/sock:dvp-vol1
func tgtLunNew(tid, lun, vdiname string) error {
	log.Debugf("Begin utils.tgtLunNew: %s, %s, %s", tid, lun, vdiname)
	bstore := "unix:/var/lib/sheepdog/sock:" + vdiname
	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "logicalunit",
		"--op", "new", "--tid", tid, "--lun", lun, "--bstype", "sheepdog",
		"--backing-store", bstore).CombinedOutput()
	log.Debug("Result of tgtLunNew: ", string(out))
	return err
}

// tgtadm --lld iscsi --mode logicalunit --op delete --tid 1 --lun 2
func tgtLunDelete(tid, lun string) error {
	// Give the suffix for Docker Volume Plugin
	log.Debugf("Begin utils.tgtLunDelete: %s, %s", tid, lun)

	out, err := exec.Command("sudo", "tgtadm", "--lld", "iscsi", "--mode", "logicalunit",
		"--op", "delete", "--tid", tid, "--lun", lun).CombinedOutput()
	log.Debug("Result of tgtLunDelete: ", string(out))
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
	exec.Command("sudo", "iscsiadm", "--mode", "session", "--rescan").CombinedOutput()
	//out, err := exec.Command("sudo", "iscsiadm", "--mode", "session", "--rescan").CombinedOutput()
	//if err != nil {
	//	log.Error("Error encountered in session rescan cmd: ", out)
	//	return false
	//}
	return true
}

// echo 1 > /sys/block/sda/device/delete
func iscsiDeleteDevice(scsi string) (err error) {
	log.Debugf("Begin utils.iscsiDeleteDevice: %s", scsi)

	cmd := "sudo echo 1 > /sys/block/" + scsi + "/device/delete"
	_, err = exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Debugf("Error during iscsi delete device: ", err)
	}
	return
}

// getDeviceNameFromLun
func getDeviceNameFromLun(tip, tport, tipn, lun string) string {
	log.Debugf("Begin utils.getDeviceNameFromLun: %s %s", tipn, lun)

	path := "/dev/disk/by-path/ip-" + tip + ":" + tport + "-iscsi-" + tipn + "-lun-" + lun

	if waitForDetectDevice(path, 5) {
		log.Debugf("volume path found: %s", path)
	}

	log.Debugf("path: %s", path)
	return path
}

// getLunFromName
func getLunFromDeviceName(vdiname string) (lun string) {
	log.Debugf("Begin utils.getLunFromDeviceName: %s", vdiname)
	// lsblk -P -S --output HCTL,TRAN,MOUNTPOINT |grep -w test1
	// HCTL="12:0:0:3" TRAN="iscsi" MOUNTPOINT="/mnt/sheepdog/test1"
	// HCTL = Host:Channel:Target:Lun
	cmd := "sudo lsblk -P -S --output HCTL,TRAN,MOUNTPOINT | grep iscsi | grep -w " + vdiname
	log.Debugf("cmd: %s", cmd)
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to get lun num: ", err)
		return
	}
	// devlist -> HCTL="12:0:0:3"
	devlist := strings.Fields(string(out))

	// pos -> 12
	pos := strings.LastIndex(devlist[0], ":")
	pos = pos + 1

	// devlist[0][pos:] -> 3"
	lun = strings.Trim(devlist[0][pos:], "\"")

	return lun
}

// getLunFromName
func getScsiNameFromDeviceName(vdiname string) (scsi string) {
	log.Debugf("Begin utils.getScsiNameFromDeviceName: %s", vdiname)
	// lsblk -P -S --output NAME,TRAN,MOUNTPOINT |grep -w test1
	// NAME="sdb" TRAN="iscsi" MOUNTPOINT="/mnt/sheepdog/test1"
	cmd := "sudo lsblk -P -S --output NAME,TRAN,MOUNTPOINT | grep iscsi | grep -w " + vdiname
	log.Debugf("cmd: %s", cmd)
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to get lun num: ", err)
		return
	}
	// devlist -> NAME="sdb"
	devlist := strings.Fields(string(out))

	// pos -> 12
	pos := strings.LastIndex(devlist[0], "=")
	pos = pos + 1

	// devlist[0][pos:] -> 3"
	scsi = strings.Trim(devlist[0][pos:], "\"")

	return scsi
}

// getDeviceFileFromIscsiPath
func getDeviceFileFromIscsiPath(iscsiPath string) (devFile string) {
	log.Debug("Begin utils.getDeviceFileFromIscsiPath: ", iscsiPath)
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

// getFSType
func getFSType(device string) string {
	log.Debugf("Begin utils.getFSType: %s", device)
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

// formatVolume
func formatVolume(device, fsType string) error {
	log.Debugf("Begin utils.formatVolume: %s, %s", device, fsType)
	cmd := "mkfs.ext4"
	if fsType == "xfs" {
		cmd = "mkfs.xfs"
	}
	log.Debug("Perform ", cmd, " on device: ", device)
	out, err := exec.Command(cmd, "-f", device).CombinedOutput()
	log.Debug("Result of mkfs cmd: ", string(out))

	return err
}

// mount
func mount(device, mountpoint string) error {
	log.Debugf("Begin utils.mount device: %s on: %s", device, mountpoint)
	out, err := exec.Command("mkdir", "-p", mountpoint).CombinedOutput()
	out, err = exec.Command("mount", device, mountpoint).CombinedOutput()
	log.Debug("Response from mount ", device, " at ", mountpoint, ": ", string(out))
	if err != nil {
		log.Error("Error in mount: ", err)
	}
	return err
}

func isAlreadyMountingThisVolume(mountpoint string) bool {
	log.Debugf("Begin utils.isAlreadyMountingThisVolume: ", mountpoint)
	// lsblk -P -S --output MOUNTPOINT  |grep -w /mnt/sheepdog/test1
	// null or line > 1
	cmd := "sudo lsblk -P -S --output MOUNTPOINT | grep -w " + mountpoint + " | wc -l"
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		log.Error("Failed to lsblk: ", err)
		return false
	}

	outInt, err := strconv.Atoi(strings.TrimRight(string(out), "\n"))
	if err != nil {
		panic(err)
	}
	if outInt != 0 {
		// mount point found, already used
		log.Debugf("mount point found, already used")
		return true
	}
	log.Debugf("mount point not found, can use it")
	return false
}

// umount
func umount(mountpoint string) error {
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

// waitForDetectDevice
func waitForDetectDevice(device string, tries int) bool {
	log.Info("Waiting for path")
	for i := 0; i < tries; i++ {
		_, err := os.Stat(device)
		if err == nil {
			log.Debug("path found: ", device)
			return true
		}
		if err != nil && !os.IsNotExist(err) {
			return false
		}
		time.Sleep(time.Second)
	}
	return false
}

// findVacantLun
func findVacantLun(tid string) (nextVacantLun string) {
	var (
		fp         *os.File
		tgtFound   int
		currLunInt int
	)
	tgtFound = 0
	nextVacantLunInt := 0

	log.Debugf("Begin utils.findVacantLun")

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
		if (tgtFound == 1) && (strings.Contains(scanner.Text(), "Target") == true) {
			break
		}
		// Check if we found the requested target
		if strings.Contains(scanner.Text(), ("Target "+tid+":")) == true {
			tgtFound = 1
			continue
		}
		if (strings.Contains(scanner.Text(), "LUN:") == true) && (tgtFound == 1) {
			currLun := strings.Fields(scanner.Text())
			currLunInt, err = strconv.Atoi(currLun[1])
			if err != nil {
				panic(err)
			}
			if currLunInt > nextVacantLunInt {
				break
			} else {
				nextVacantLunInt = nextVacantLunInt + 1
			}
		}
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}

	nextVacantLun = strconv.Itoa(nextVacantLunInt)
	return nextVacantLun
}
