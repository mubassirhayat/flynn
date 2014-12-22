package zfs

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"time"

	zfs "github.com/flynn/flynn/Godeps/_workspace/src/github.com/mistifyio/go-zfs"
	"github.com/flynn/flynn/host/volume"
	"github.com/flynn/flynn/pkg/random"
)

type zfsVolume struct {
	info      *volume.Info
	mounts    map[volume.VolumeMount]struct{}
	// FIXME: starting to look better to put this back in the hands of the provider, and making all of the challenges of maintaining entanglement with external state confined to that.
	poolName  string // The name of the zpool this storage is cut from.  (We need this when forking snapshots, or doing some inspections.)
	basemount string // This is the location of the main mount of the ZFS dataset.  Mounts into containers are bind-mounts pointing back out to this.  The user does not control it (it is essentially an implementation detail).
}

type Provider struct {
	config *ProviderConfig
}

/*
	Stores zfs config used at setup time.

	`volume.ProviderSpec.Config` is deserialized to this for zfs.
*/
type ProviderConfig struct {
	Vdev string // TODO: this is going to be complicated in itself; it's either a dataset name from a presumed-existing zpool, or a request to do file provisioning
	// on the plus side, i don't think we have any earthly reason to support zpool creation directly (if you have a fancy hardware setup, you're likely able to manage that yourself).
	DatasetName string
}

func NewProvider(config *ProviderConfig) (volume.Provider, error) {
	if _, err := exec.LookPath("zfs"); err != nil {
		return nil, fmt.Errorf("zfs command is not available")
	}
	return &Provider{
		config: config,
	}, nil
}

func (b Provider) NewVolume() (volume.Volume, error) {
	id := random.UUID()
	v := &zfsVolume{
		info:      &volume.Info{ID: id},
		mounts:    make(map[volume.VolumeMount]struct{}),
		poolName:  b.config.DatasetName,
		basemount: filepath.Join("/var/lib/flynn/volumes/zfs/", id),
	}
	if _, err := zfs.CreateFilesystem(path.Join(v.poolName, id), map[string]string{
		"mountpoint": v.basemount,
	}); err != nil {
		return nil, err
	}
	return v, nil
}

func (v *zfsVolume) Info() *volume.Info {
	return v.info
}

func (v *zfsVolume) Mounts() map[volume.VolumeMount]struct{} {
	return v.mounts
}

func (v *zfsVolume) Mount(jobId, path string) (string, error) {
	mount := volume.VolumeMount{
		JobID:    jobId,
		Location: path,
	}
	if _, exists := v.mounts[mount]; exists {
		return "", fmt.Errorf("volume: cannot make same mount twice!")
	}
	v.mounts[mount] = struct{}{}
	return v.basemount, nil
}

func (v1 *zfsVolume) TakeSnapshot() (volume.Volume, error) {
	id := random.UUID()
	v2 := &zfsVolume{
		info:      &volume.Info{ID: id},
		mounts:    make(map[volume.VolumeMount]struct{}),
		poolName:  v1.poolName,
		basemount: filepath.Join("/var/lib/flynn/volumes/zfs/", id),
	}
	if err := cloneFilesystem(path.Join(v2.poolName, v2.info.ID), path.Join(v1.poolName, v1.info.ID), v2.basemount); err != nil {
		return nil, err
	}
	return v2, nil
}

func cloneFilesystem(newDatasetName string, parentDatasetName string, mountPath string) error {
	parentDataset, err := zfs.GetDataset(parentDatasetName)
	if parentDataset == nil {
		return err
	}
	snapshotName := fmt.Sprintf("%d", time.Now().Nanosecond())
	snapshot, err := parentDataset.Snapshot(snapshotName, false)
	if err != nil {
		return err
	}

	_, err = snapshot.Clone(newDatasetName, map[string]string{
		"mountpoint": mountPath,
	})
	if err != nil {
		snapshot.Destroy(zfs.DestroyDeferDeletion)
		return err
	}
	err = snapshot.Destroy(zfs.DestroyDeferDeletion)
	return err
}
