package service

import (
	"context"
	"errors"
	"testing"
	"time"

	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPassthroughClientControlBeforeOutputDisablesCapacityRetry(t *testing.T) {
	for _, control := range []string{`{"type":"response.cancel"}`, `{"type":"session.update","session":{"model":"gpt-5.1"}}`} {
		t.Run(gjson.Get(control, "type").String(), func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			upstream := newStagedPassthroughConn()
			upstream.Send(`{"type":"response.created","response":{"id":"controlled"}}`)
			server, serverErr := startPassthroughLifecycleServer(t, context.Background(), newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount())
			defer server.Close()
			client := dialPassthroughLifecycleClient(t, server)
			defer func() { _ = client.CloseNow() }()
			require.Equal(t, "response.create", gjson.GetBytes(requirePassthroughUpstreamWrite(t, upstream, time.Second), "type").String())
			writeCtx, cancelWrite := context.WithTimeout(context.Background(), time.Second)
			defer cancelWrite()
			require.NoError(t, client.Write(writeCtx, coderws.MessageText, []byte(control)))
			require.JSONEq(t, control, string(requirePassthroughUpstreamWrite(t, upstream, time.Second)))
			upstream.Send(`{"type":"response.failed","response":{"id":"controlled","error":{"code":"server_is_overloaded","message":"busy"}}}`)
			for _, wantType := range []string{"response.created", "response.failed"} {
				payload, err := readPassthroughLifecycleFrame(t, client, time.Second)
				require.NoError(t, err)
				require.Equal(t, wantType, gjson.GetBytes(payload, "type").String())
			}
			require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
			select {
			case err := <-serverErr:
				var retry *UpstreamFailoverError
				require.False(t, errors.As(err, &retry), "client control retires transparent replay")
			case <-time.After(3 * time.Second):
				t.Fatal("controlled attempt did not stop")
			}
			require.Empty(t, upstream.writes, "no response.create may replay after client control")
		})
	}
}

func TestPassthroughLaterTurnCancelPreservesResponseID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := newStagedPassthroughConn()
	upstream.Send(`{"type":"response.completed","response":{"id":"first","usage":{"input_tokens":1,"output_tokens":1}}}`)
	server, serverErr := startPassthroughLifecycleServer(t, context.Background(), newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream), passthroughLifecycleAccount())
	defer server.Close()
	client := dialPassthroughLifecycleClient(t, server)
	defer func() { _ = client.CloseNow() }()
	_ = requirePassthroughUpstreamWrite(t, upstream, time.Second)
	_, err := readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, err)
	writeCtx, cancelWrite := context.WithTimeout(context.Background(), time.Second)
	defer cancelWrite()
	require.NoError(t, client.Write(writeCtx, coderws.MessageText, []byte(`{"type":"response.create","model":"gpt-5.1","previous_response_id":"first"}`)))
	_ = requirePassthroughUpstreamWrite(t, upstream, time.Second)
	upstream.Send(`{"type":"response.created","response":{"id":"second"}}`)
	_, err = readPassthroughLifecycleFrame(t, client, time.Second)
	require.NoError(t, err)
	control := `{"type":"response.cancel","response_id":"second"}`
	require.NoError(t, client.Write(writeCtx, coderws.MessageText, []byte(control)))
	require.JSONEq(t, control, string(requirePassthroughUpstreamWrite(t, upstream, time.Second)))
	require.NoError(t, client.Close(coderws.StatusNormalClosure, "done"))
	select {
	case <-serverErr:
	case <-time.After(3 * time.Second):
		t.Fatal("later-turn cancellation did not stop")
	}
}
