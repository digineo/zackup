package app

import (
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "app")

var (
	// RootDataset is the name of the ZFS dataset under which zackup
	// creates per-host datasets.
	RootDataset = "zpool"

	// MountBase is the name of the directory which zackup uses to
	// mount per-host datasets for rsync.
	//
	// A special directory (MountBase/.zackup) is used as working
	// directory for temp. files, such as SSH ControlPath sockets.
	MountBase = "/zackup"
)
