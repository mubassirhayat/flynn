package zfs

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/flynn/flynn/Godeps/_workspace/src/github.com/flynn/go-check"
)

func Test(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

/*
option 1
--------

apt-get install zfs-fuse


option 2
--------

gpg --keyserver pgp.mit.edu --recv-keys "F6B0FC61"
gpg --armor --export "F6B0FC61" | apt-key add -
echo deb http://ppa.launchpad.net/zfs-native/stable/ubuntu trusty main > /etc/apt/sources.list.d/zfs.list
apt-get update
apt-get install -y ubuntu-zfs

this drags in g++, and proceeds to spend a large number of seconds on "Building initial module for 3.13.0-43-generic"...
okay, minutes.  ugh.
but, this one gives the better experience and much better performance.


sample commands to set up a zpool
---------------------------------

dd if=/dev/zero of=/tmp/zvdev count=1k bs=64k
zpool create demopool -mnone /tmp/zvdev
zfs create -o mountpoint=/tmp/zdemo demopool/zdemo
# now inspect with:
ls -la /tmp/zdemo
zpool list
zfs list
# later:
zfs destroy -d demopool/zdemo # well... the go-zfs package does this, but it doesn't appear to do much.  """cannot open 'demopool/zdemo': operation not applicable to datasets of this type""" when I do it manually.  maybe should file a bug for that.
zpool destroy demopool

# note: you can force unmounting a "busy" filesystem with `umount -l` (not '-f' as one might normally expect).
# this may leave processes in odd states, still able to write to files, though those files are invisible if you're `ls`'ing around
# ohhhhh boy.  no, this is just another form of lie.  that's a "lazy" unmount -- it leaves the filesystem busy nonetheless.  after that you can't even remount it, comically.

*/

//

func (S) TestSnapshotShouldCarryFiles(c *C) {
	err := WithTmpfileZpool("testpool", func() error {
		provider, err := NewProvider(&ProviderConfig{DatasetName: "testpool"})
		if err != nil {
			return err
		}

		v, err := provider.NewVolume()
		if err != nil {
			return err
		}

		// a new volume should start out empty:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{})

		f, err := os.Create(filepath.Join(v.(*zfsVolume).basemount, "alpha"))
		c.Assert(err, IsNil)
		f.Close()

		// sanity check, can we so much as even write a file:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{"alpha"})

		v2, err := v.TakeSnapshot()
		if err != nil {
			return err
		}

		// taking a snapshot shouldn't change the source dir:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{"alpha"})
		// the newly mounted snapshot in the new location should have the same content:
		c.Assert(v2.(*zfsVolume).basemount, DirContains, []string{"alpha"})

		return nil
	})
	c.Assert(err, IsNil)
}

func (S) TestSnapshotShouldIsolateNewChangesToSource(c *C) {
	err := WithTmpfileZpool("testpool", func() error {
		provider, err := NewProvider(&ProviderConfig{DatasetName: "testpool"})
		if err != nil {
			return err
		}

		v, err := provider.NewVolume()
		if err != nil {
			return err
		}

		// a new volume should start out empty:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{})

		f, err := os.Create(filepath.Join(v.(*zfsVolume).basemount, "alpha"))
		c.Assert(err, IsNil)
		f.Close()

		// sanity check, can we so much as even write a file:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{"alpha"})

		v2, err := v.TakeSnapshot()
		if err != nil {
			return err
		}

		// write another file to the source
		f, err = os.Create(filepath.Join(v.(*zfsVolume).basemount, "beta"))
		c.Assert(err, IsNil)
		f.Close()

		// the source dir should contain our changes:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{"alpha", "beta"})
		// the snapshot should be unaffected:
		c.Assert(v2.(*zfsVolume).basemount, DirContains, []string{"alpha"})

		return nil
	})
	c.Assert(err, IsNil)
}

func (S) TestSnapshotShouldIsolateNewChangesToFork(c *C) {
	err := WithTmpfileZpool("testpool", func() error {
		provider, err := NewProvider(&ProviderConfig{DatasetName: "testpool"})
		if err != nil {
			return err
		}

		v, err := provider.NewVolume()
		if err != nil {
			return err
		}

		// a new volume should start out empty:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{})

		f, err := os.Create(filepath.Join(v.(*zfsVolume).basemount, "alpha"))
		c.Assert(err, IsNil)
		f.Close()

		// sanity check, can we so much as even write a file:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{"alpha"})

		v2, err := v.TakeSnapshot()
		if err != nil {
			return err
		}

		// write another file to the fork
		f, err = os.Create(filepath.Join(v2.(*zfsVolume).basemount, "beta"))
		c.Assert(err, IsNil)
		f.Close()

		// the source dir should be unaffected:
		c.Assert(v.(*zfsVolume).basemount, DirContains, []string{"alpha"})
		// the snapshot should contain our changes:
		c.Assert(v2.(*zfsVolume).basemount, DirContains, []string{"alpha", "beta"})

		return nil
	})
	c.Assert(err, IsNil)
}
