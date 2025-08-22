package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/henrywhitaker3/windowframe/queue"
	"github.com/henrywhitaker3/windowframe/queue/nats"
	"golang.org/x/time/rate"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	conn, err := nats.NewConsumer(nats.ConsumerOpts{
		URL:        "10.0.0.39:4222",
		StreamName: "demo",
	})
	if err != nil {
		panic(err)
	}

	cons := queue.NewConsumer(queue.ConsumerOpts{
		Consumer: conn,
		Observer: queue.NewObserver(queue.ObserverOpts{
			Logger: slog.Default(),
		}),
	})

	cons.RegisterHandler(queue.Task("bongo"), func(ctx context.Context, payload []byte) error {
		return nil
	})

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if slices.Contains(os.Args, "--consume") {
		go func() {
			if err := cons.Consume(ctx); err != nil {
				panic(err)
			}
		}()
	}
	defer cons.Close(context.Background())

	prod, err := nats.NewProducer(nats.ProducerOpts{
		URL: "10.0.0.39:4222",
	})
	if err != nil {
		panic(err)
	}

	p := queue.NewProducer(queue.ProducerOpts{
		Producer: prod,
		Observer: queue.NewObserver(queue.ObserverOpts{
			Logger: slog.Default(),
		}),
	})

	if slices.Contains(os.Args, "--push") {
		lim := rate.NewLimiter(1000, 1)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				_ = lim.Wait(ctx)
				if err := p.Push(
					ctx,
					queue.Task("bongo"),
					fmt.Sprintf("pushed at %s", time.Now().Format(time.RFC3339)),
					queue.OnQueue(queue.Queue("demo")),
				); err != nil {
					slog.Error("failed to push job", "error", err)
				}
			}
		}
	}
	<-ctx.Done()
}
