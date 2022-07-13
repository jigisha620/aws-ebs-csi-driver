//go:build linux
// +build linux

/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package driver

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
	"k8s.io/klog"
)

type BlockDevice struct {
	Name       string `json:"name,omitempty"`
	MountPoint string `json:"mountpoint,omitempty"`
}

func (d *nodeService) appendPartition(devicePath, partition string) string {
	if partition == "" {
		return devicePath
	}

	if strings.HasPrefix(devicePath, "/dev/nvme") {
		return devicePath + nvmeDiskPartitionSuffix + partition
	}

	return devicePath + diskPartitionSuffix + partition
}

// findDevicePath finds path of device and verifies its existence
// if the device is not nvme, return the path directly
// if the device is nvme, finds and returns the nvme device path eg. /dev/nvme1n1
func (d *nodeService) findDevicePath(devicePath, volumeID, partition string) (string, error) {
	canonicalDevicePath := ""

	// If the given path exists, the device MAY be nvme. Further, it MAY be a
	// symlink to the nvme device path like:
	// | $ stat /dev/xvdba
	// | File: ‘/dev/xvdba’ -> ‘nvme1n1’
	// Since these are maybes, not guarantees, the search for the nvme device
	// path below must happen and must rely on volume ID
	exists, err := d.mounter.PathExists(devicePath)
	if err != nil {
		return "", fmt.Errorf("failed to check if path %q exists: %v", devicePath, err)
	}

	if exists {
		stat, err := d.deviceIdentifier.Lstat(devicePath)
		if err != nil {
			return "", fmt.Errorf("failed to lstat %q: %v", devicePath, err)
		}

		if stat.Mode()&os.ModeSymlink == os.ModeSymlink {
			canonicalDevicePath, err = d.deviceIdentifier.EvalSymlinks(devicePath)
			if err != nil {
				return "", fmt.Errorf("failed to evaluate symlink %q: %v", devicePath, err)
			}
		} else {
			canonicalDevicePath = devicePath
		}

		klog.V(5).Infof("[Debug] The canonical device path for %q was resolved to: %q", devicePath, canonicalDevicePath)
		return d.appendPartition(canonicalDevicePath, partition), nil
	}

	klog.V(5).Infof("[Debug] Falling back to nvme volume ID lookup for: %q", devicePath)

	// AWS recommends identifying devices by volume ID
	// (https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/nvme-ebs-volumes.html),
	// so find the nvme device path using volume ID. This is the magic name on
	// which AWS presents NVME devices under /dev/disk/by-id/. For example,
	// vol-0fab1d5e3f72a5e23 creates a symlink at
	// /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol0fab1d5e3f72a5e23
	nvmeName := "nvme-Amazon_Elastic_Block_Store_" + strings.Replace(volumeID, "-", "", -1)

	nvmeDevicePath, err := findNvmeVolume(d.deviceIdentifier, nvmeName)

	if err == nil {
		klog.V(5).Infof("[Debug] successfully resolved nvmeName=%q to %q", nvmeName, nvmeDevicePath)
		canonicalDevicePath = d.appendPartition(nvmeDevicePath, partition)
		return canonicalDevicePath, nil
	} else {
		klog.V(5).Infof("[Debug] error searching for nvme path %q: %v", nvmeName, err)
	}

	klog.V(5).Infof("[Debug] Falling back to snow volume lookup for: %q", devicePath)

	//snowDevicePath, err := d.deviceIdentifier.FindSnowVolume()
	//snowDevicePath, err := d.deviceIdentifier.Lstat("/dev/vda")
	snowDevicePath, err := findSnowVolume(d, err)

	//if err == nil {
	//	klog.V(5).Infof("[Debug] successfully resolved devicePath=%q to %q", devicePath, snowDevicePath)
	//	canonicalDevicePath = snowDevicePath
	//} else {
	//	klog.V(5).Infof("[Debug] error searching for snow path: %v", err)
	//}

	if canonicalDevicePath == "" {
		return "", errNoDevicePathFound(devicePath, volumeID, snowDevicePath, err)
	}

	canonicalDevicePath = d.appendPartition(canonicalDevicePath, partition)
	return canonicalDevicePath, nil
}

func findSnowVolume(d *nodeService, err error) ([]byte, error) {
	//snowDevicePath := ""
	//cmd := d.mounter.(*NodeMounter).Exec.Command("/usr/bin/lsblk", "--json")
	//output, err := cmd.Output()
	cmd := osexec.Command("lsblk", "--json")
	output, err := cmd.Output()
	//rawOut := make(map[string][]BlockDevice, 1)
	//err = json.Unmarshal(output, &rawOut)
	//if err != nil {
	//	klog.V(5).Infof("unable to unmarshal output to BlockDevice instance, error: %v", err)
	//}
	//var (
	//	devs []BlockDevice
	//	ok   bool
	//)
	//if devs, ok = rawOut["blockdevices"]; !ok {
	//	klog.V(5).Infof("unexpected lsblk output format, missing block devices")
	//}
	//for _, d := range devs {
	//	if (strings.HasPrefix(d.Name, "/dev/v")) && (len(d.MountPoint) == 0) {
	//		snowDevicePath = d.Name
	//	}
	//}
	//return snowDevicePath, err
	return output, err
}

func errNoDevicePathFound(devicePath string, volumeID string, snowDevicePath []byte, err error) error {
	return fmt.Errorf("no device path for device %q volume %q found snowdevicePath %v errorMount %v", devicePath, volumeID, snowDevicePath, err)
}

//func errNoDevicePathFound(devicePath, volumeID string) error {
//	return fmt.Errorf("no device path for device %q volume %q found", devicePath, volumeID)
//}

// findNvmeVolume looks for the nvme volume with the specified name
// It follows the symlink (if it exists) and returns the absolute path to the device
func findNvmeVolume(deviceIdentifier DeviceIdentifier, findName string) (device string, err error) {
	p := filepath.Join("/dev/disk/by-id/", findName)
	stat, err := deviceIdentifier.Lstat(p)
	if err != nil {
		if os.IsNotExist(err) {
			klog.V(5).Infof("[Debug] nvme path %q not found", p)
			return "", fmt.Errorf("nvme path %q not found", p)
		}
		return "", fmt.Errorf("error getting stat of %q: %v", p, err)
	}

	if stat.Mode()&os.ModeSymlink != os.ModeSymlink {
		klog.Warningf("nvme file %q found, but was not a symlink", p)
		return "", fmt.Errorf("nvme file %q found, but was not a symlink", p)
	}
	// Find the target, resolving to an absolute path
	// For example, /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol0fab1d5e3f72a5e23 -> ../../nvme2n1
	resolved, err := deviceIdentifier.EvalSymlinks(p)
	if err != nil {
		return "", fmt.Errorf("error reading target of symlink %q: %v", p, err)
	}

	if !strings.HasPrefix(resolved, "/dev") {
		return "", fmt.Errorf("resolved symlink for %q was unexpected: %q", p, resolved)
	}

	return resolved, nil
}

func (d *nodeService) preparePublishTarget(target string) error {
	klog.V(4).Infof("NodePublishVolume: creating dir %s", target)
	if err := d.mounter.MakeDir(target); err != nil {
		return fmt.Errorf("Could not create dir %q: %v", target, err)
	}
	return nil
}

// IsBlock checks if the given path is a block device
func (d *nodeService) IsBlockDevice(fullPath string) (bool, error) {
	var st unix.Stat_t
	err := unix.Stat(fullPath, &st)
	if err != nil {
		return false, err
	}

	return (st.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

func (d *nodeService) getBlockSizeBytes(devicePath string) (int64, error) {
	cmd := d.mounter.(*NodeMounter).Exec.Command("blockdev", "--getsize64", devicePath)
	output, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", devicePath, string(output), err)
	}
	strOut := strings.TrimSpace(string(output))
	gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
	if err != nil {
		return -1, fmt.Errorf("failed to parse size %s as int", strOut)
	}
	return gotSizeBytes, nil
}
