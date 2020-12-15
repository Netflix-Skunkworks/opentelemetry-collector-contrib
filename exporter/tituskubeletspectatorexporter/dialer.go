package tituskubeletspectatorexporter

import (
	"context"
	"errors"
	"net"
	"os"
	"runtime"
	"time"

	pkgerrors "github.com/pkg/errors"
	"go.uber.org/multierr"

	"github.com/vishvananda/netns"
)

type NsDialer struct {
	netNsPath string
	dialer    net.Dialer
}

var (
	ErrorUnknownContainer = errors.New("Unknown container")
)

func NewNsDialer(netNsPath string) (*NsDialer, error) {
	_, err := os.Stat(netNsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrorUnknownContainer
		}

		return nil, err
	}

	return &NsDialer{
		netNsPath: netNsPath,
		dialer: net.Dialer{
			Timeout:       30 * time.Second,
			KeepAlive:     30 * time.Second,
			FallbackDelay: -1, // Disable IPv4 fallback
		},
	}, nil
}

func (n *NsDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	runtime.LockOSThread()
	defer func() {
		runtime.UnlockOSThread()
	}()

	origNs, err := netns.Get()
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Unable to save namespace")
	}
	defer func() {
		_ = origNs.Close()
	}()

	dialNs, err := netns.GetFromPath(n.netNsPath)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Unable to open namespace")
	}
	defer func() {
		_ = dialNs.Close()
	}()

	err = netns.Set(dialNs)
	if err != nil {
		return nil, pkgerrors.Wrap(err, "Unable to enter namespace")
	}

	conn, err := n.dialer.DialContext(ctx, network, address)

	errA := netns.Set(origNs)
	if errA != nil {
		errA = pkgerrors.Wrap(err, "Unable to restore namespace")
	}

	return conn, multierr.Combine(err, errA)
}
