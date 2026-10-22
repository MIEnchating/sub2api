package service

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMonitorRejectsMulticastDestinations(t *testing.T) {
	for _, address := range []string{"224.0.0.1", "239.255.255.250", "::ffff:239.255.255.250", "ff05::1", "ff0e::1"} {
		t.Run(address, func(t *testing.T) {
			require.True(t, isPrivateIP(net.ParseIP(address)))
			blocked, err := isPrivateOrLoopbackHost(t.Context(), address)
			require.NoError(t, err)
			require.True(t, blocked)
			conn, err := safeDialContext(t.Context(), "tcp", net.JoinHostPort(address, "443"))
			require.ErrorContains(t, err, "blocked")
			require.Nil(t, conn)
		})
	}
	for _, address := range []string{"8.8.8.8", "2606:4700:4700::1111"} {
		require.False(t, isPrivateIP(net.ParseIP(address)), "public unicast addresses remain allowed")
	}
}
