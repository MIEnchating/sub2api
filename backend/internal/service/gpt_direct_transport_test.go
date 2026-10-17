package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGPTDirectDoesNotReplayRedemptionAfterConnectionLoss(t *testing.T) {
	settings := NewSettingService(&gptDirectSettingsRepo{values: map[string]string{}}, nil)
	_, err := settings.SaveGPTDirectConfig(context.Background(), GPTDirectConfig{
		Enabled: true, BaseURL: "http://direct.invalid", TenantSlug: "example-agent", PurchaseURL: "https://shop.example.com/cdk",
	})
	require.NoError(t, err)
	client := NewGPTDirectService(settings)
	var redemptions, connections atomic.Int32
	transport := &http.Transport{
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			connections.Add(1)
			local, peer := net.Pipe()
			deadline := time.Now().Add(5 * time.Second)
			_ = local.SetDeadline(deadline)
			_ = peer.SetDeadline(deadline)
			go func() {
				defer func() { _ = peer.Close() }()
				reader := bufio.NewReader(peer)
				for {
					request, err := http.ReadRequest(reader)
					if err != nil {
						return
					}
					_, err = io.Copy(io.Discard, request.Body)
					_ = request.Body.Close()
					if err != nil {
						return
					}
					body := `{"valid":true,"product":"gpt","plan":"plus"}`
					if request.URL.Path == "/api/v1/public/redeem" {
						if redemptions.Add(1) == 1 {
							// The provider accepted the write but lost the response on
							// a reused connection. The transport must not replay it.
							return
						}
						body = fmt.Sprintf(`{"task":{"id":"task_1","status":"queued","product":"gpt"},"token":%q}`, strings.Repeat("a", 43))
					}
					if _, err := fmt.Fprintf(peer, "HTTP/1.1 200 OK\r\nContent-Length: %d\r\nContent-Type: application/json\r\n\r\n%s", len(body), body); err != nil {
						return
					}
				}
			}()
			return local, nil
		},
	}
	t.Cleanup(transport.CloseIdleConnections)
	client.client.Transport = transport
	_, failure := client.Call(context.Background(), "verify", GPTDirectInput{Code: "CODE-123"}, 7, "", "")
	require.Nil(t, failure)
	require.Equal(t, int32(1), connections.Load())
	_, failure = client.Call(context.Background(), "redeem", GPTDirectInput{Code: "CODE-123", Session: "test-session"}, 7, "", "same-request-id")
	require.Equal(t, int32(1), redemptions.Load(), "an ambiguous write must require an explicit user retry")
	require.NotNil(t, failure)
	require.Equal(t, "UPSTREAM_UNAVAILABLE", failure.Reason)
	// An explicit user retry can still recover the receipt with the same key.
	_, failure = client.Call(context.Background(), "redeem", GPTDirectInput{Code: "CODE-123", Session: "test-session"}, 7, "", "same-request-id")
	require.Nil(t, failure)
	require.Equal(t, int32(2), redemptions.Load())
}
