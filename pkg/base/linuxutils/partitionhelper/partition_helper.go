// Package partitionhelper contains code for manipulating with block device partitions and
// run such system utilites as parted, partprobe, sgdisk
package partitionhelper

import (
	"fmt"
	"strings"

	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/command"
	"eos2git.cec.lab.emc.com/ECS/baremetal-csi-plugin.git/pkg/base/util"
)

// Partitioner is the interface which encapsulates methods to work with drives' partitions
type Partitioner interface {
	IsPartitionExists(device, partNum string) (exists bool, err error)
	GetPartitionTableType(device string) (ptType string, err error)
	CreatePartitionTable(device, partTableType string) (err error)
	CreatePartition(device, partName string) (err error)
	DeletePartition(device, partNum string) (err error)
	SetPartitionUUID(device, partNum, partUUID string) error
	GetPartitionUUID(device, partNum string) (string, error)
	SyncPartitionTable(device string) error
}

const (
	// PartitionGPT is the const for GPT partition table
	PartitionGPT = "gpt"
	// parted is a name of system util
	parted = "parted "
	// partprobe is a name of system util
	partprobe = "partprobe "
	// sgdisk is a name of system util
	sgdisk = "sgdisk "

	// PartprobeDeviceCmdTmpl check that device has partition cmd
	PartprobeDeviceCmdTmpl = partprobe + "-d -s %s"
	// PartprobeCmdTmpl check device has partition with partprobe cmd
	PartprobeCmdTmpl = partprobe + "%s"

	// CreatePartitionTableCmdTmpl create partition table on provided device of provided type cmd template
	// fill device and partition table type
	CreatePartitionTableCmdTmpl = parted + "-s %s mklabel %s"
	// CreatePartitionCmdTmpl create partition on provided device cmd template, fill device and partition name
	CreatePartitionCmdTmpl = parted + "-s %s mkpart --align optimal %s 0%% 100%%"
	// DeletePartitionCmdTmpl delete partition from provided device cmd template, fill device and partition number
	DeletePartitionCmdTmpl = parted + "-s %s rm %s"

	// SetPartitionUUIDCmdTmpl command for set GUID of the partition, fill device, part number and part UUID
	SetPartitionUUIDCmdTmpl = sgdisk + "%s --partition-guid=%s:%s"
	// GetPartitionUUIDCmdTmpl command for read GUID of the first partition, fill device and part number
	GetPartitionUUIDCmdTmpl = sgdisk + "%s --info=%s"
)

// supportedTypes list of supported partition table types
var supportedTypes = []string{PartitionGPT}

// Partition is the basic implementation of Partitioner interface
type Partition struct {
	e command.CmdExecutor
}

// NewPartition is a constructor for Partition instance
func NewPartition(e command.CmdExecutor) *Partition {
	return &Partition{
		e: e,
	}
}

// IsPartitionExists checks if a partition exists in a provided device
// Receives path to a device to check a partition existence
// Returns partition existence status or error if something went wrong
func (p *Partition) IsPartitionExists(device, partNum string) (bool, error) {
	cmd := fmt.Sprintf(PartprobeDeviceCmdTmpl, device)
	/*
		example of output:
		$ partprobe -d -s /dev/sdy
		/dev/sdy: gpt partitions 1 2
	*/

	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return false, fmt.Errorf("unable to check partition %#v existence for %s", partNum, device)
	}

	stdout = strings.TrimSpace(stdout)

	s := strings.Split(stdout, "partitions")
	// after splitting partition number might appear on 2nd place in slice
	if len(s) > 1 && s[1] != "" {
		return true, nil
	}

	return false, nil
}

// CreatePartitionTable created partition table on a provided device
// Receives device path on which to create table
// Returns error if something went wrong
func (p *Partition) CreatePartitionTable(device, partTableType string) error {
	if !util.ContainsString(supportedTypes, partTableType) {
		return fmt.Errorf("unable to create partition table for device %s unsupported partition table type: %#v",
			device, partTableType)
	}

	cmd := fmt.Sprintf(CreatePartitionTableCmdTmpl, device, partTableType)
	_, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return fmt.Errorf("unable to create partition table for device %s", device)
	}

	return nil
}

// GetPartitionTableType returns string that represent partition table type
// Receives device path from which partition table type should be got
// Returns partition table type as a string or error if something went wrong
func (p *Partition) GetPartitionTableType(device string) (string, error) {
	cmd := fmt.Sprintf(PartprobeDeviceCmdTmpl, device)

	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return "", fmt.Errorf("unable to get partition table for device %s", device)
	}
	// /dev/sda: msdos partitions 1
	s := strings.Split(stdout, " ")
	if len(s) < 2 {
		return "", fmt.Errorf("unable to parse output '%s' for device %s", stdout, device)
	}
	// partition table type is on 2nd place in slice
	return s[1], nil
}

// CreatePartition creates partition with name partName on a device
// Receives device path to create a partition
// Returns error if something went wrong
func (p *Partition) CreatePartition(device, partName string) error {
	cmd := fmt.Sprintf(CreatePartitionCmdTmpl, device, partName)

	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}

	return nil
}

// DeletePartition removes partition partNum from a provided device
// Receives device path and it's partition which should be deleted
// Returns error if something went wrong
func (p *Partition) DeletePartition(device, partNum string) error {
	cmd := fmt.Sprintf(DeletePartitionCmdTmpl, device, partNum)

	if _, stderr, err := p.e.RunCmd(cmd); err != nil {
		return fmt.Errorf("unable to delete partition %#v from device %s: %s, error: %v",
			partNum, device, stderr, err)
	}

	return nil
}

// SetPartitionUUID writes partUUID as GUID for the partition partNum of a provided device
// Receives device path and partUUID as strings
// Returns error if something went wrong
func (p *Partition) SetPartitionUUID(device, partNum, partUUID string) error {
	cmd := fmt.Sprintf(SetPartitionUUIDCmdTmpl, device, partNum, partUUID)

	if _, _, err := p.e.RunCmd(cmd); err != nil {
		return err
	}

	return nil
}

// GetPartitionUUID reads partition unique GUID from the partition partNum of a provided device
// Receives device path from which to read
// Returns unique GUID as a string or error if something went wrong
func (p *Partition) GetPartitionUUID(device, partNum string) (string, error) {
	/*
		example of command output:
		$ sgdisk /dev/sdy --info=1
		Partition GUID code: 0FC63DAF-8483-4772-8E79-3D69D8477DE4 (Linux filesystem)
		Partition unique GUID: 5209CFD8-3AB1-4720-BCEA-DFA80315EC92
		First sector: 2048 (at 1024.0 KiB)
		Last sector: 999423 (at 488.0 MiB)
		Partition size: 997376 sectors (487.0 MiB)
		Attribute flags: 0000000000000000
		Partition name: ''
	*/
	cmd := fmt.Sprintf(GetPartitionUUIDCmdTmpl, device, partNum)
	partitionPresentation := "Partition unique GUID:"

	stdout, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(stdout, "\n") {
		if strings.Contains(line, partitionPresentation) {
			res := strings.Split(strings.TrimSpace(line), partitionPresentation)
			if len(res) > 1 {
				return strings.ToLower(strings.TrimSpace(res[1])), nil
			}
		}
	}

	return "", fmt.Errorf("unable to get partition GUID for device %s", device)
}

// SyncPartitionTable syncs partition table for specific device
// Receives device path to sync with partprobe, device could be an empty string (sync for all devices in the system)
// Returns error if something went wrong
func (p *Partition) SyncPartitionTable(device string) error {
	cmd := fmt.Sprintf(PartprobeCmdTmpl, device)

	_, _, err := p.e.RunCmd(cmd)

	if err != nil {
		return err
	}

	return nil
}