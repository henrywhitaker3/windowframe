package comms_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/henrywhitaker3/windowframe/comms"
	"github.com/henrywhitaker3/windowframe/test"
	"github.com/stretchr/testify/require"
)

func TestItRespondsToMessages(t *testing.T) {
	send, _, cancel := setup(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	_, err := send.Send(ctx, comms.Topic("example"), []byte("bongo"))
	require.NotNil(t, err)

	reply, err := send.Send(ctx, comms.Topic("test"), []byte("bongo"))
	require.Nil(t, err)
	require.Equal(t, "BONGO", string(reply))
}

func TestItForwardsErrors(t *testing.T) {
	send, respond, cancel := setup(t)
	defer cancel()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	go func() {
		if err := respond.Listen(ctx, comms.Topic("example"), func(ctx context.Context, b []byte) ([]byte, error) {
			return nil, fmt.Errorf("something went wrong")
		}); err != nil {
			panic(err)
		}
	}()
	time.Sleep(time.Second)

	_, err := send.Send(ctx, comms.Topic("example"), nil)
	require.NotNil(t, err)
	require.Equal(t, "something went wrong", err.Error())
}

func setup(t *testing.T) (*comms.Sender, *comms.Responder, context.CancelFunc) {
	nats, cancelNats := test.Nats(t)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)

	send, err := comms.NewSender(comms.SenderOpts{
		URL: nats,
	})
	require.Nil(t, err)

	respond, err := comms.NewResponder(comms.ResponderOpts{
		URL: nats,
	})
	require.Nil(t, err)

	go func() {
		if err := respond.Listen(
			ctx,
			comms.Topic("test"),
			func(ctx context.Context, b []byte) ([]byte, error) {
				return []byte(strings.ToUpper(string(b))), nil
			},
		); err != nil {
			panic(err)
		}
	}()

	time.Sleep(time.Second)

	return send, respond, func() {
		cancel()
		cancelNats()
	}
}
