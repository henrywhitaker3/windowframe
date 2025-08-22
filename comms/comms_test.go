package comms_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/comms"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/stretchr/testify/require"
)

func TestItRespondsToMessages(t *testing.T) {
	nats, cancel := test.Nats(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	send, err := comms.NewSender(comms.SenderOpts{
		URL: nats,
	})
	require.Nil(t, err)

	respond, err := comms.NewResponder(comms.ResponderOpts{
		URL: nats,
	})
	require.Nil(t, err)

	_, err = send.Send(ctx, comms.Topic("example"), []byte("bongo"))
	require.NotNil(t, err)

	go func() {
		if err := respond.Listen(
			ctx,
			comms.Topic("example"),
			func(ctx context.Context, b []byte) ([]byte, error) {
				return []byte(strings.ToUpper(string(b))), nil
			},
		); err != nil {
			panic(err)
		}
	}()

	time.Sleep(time.Second)

	reply, err := send.Send(ctx, comms.Topic("example"), []byte("bongo"))
	require.Nil(t, err)
	require.Equal(t, "BONGO", string(reply))
}
