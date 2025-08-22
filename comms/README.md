# Comms

Enabled two-way communication via an intermediate broker (e.g. NATS, Redis)

## Usage

```go
send, err := comms.NewSender(comms.SenderOpts{
    URL: "some_url",
})
if err != nil {
    panic(err)
}

respond, err := comms.NewResponder(comms.ResponderOpts{
    URL: "some_url",
})
if err != nil {
    panic(err)
}

go respond.Listen(
    context.Background(), 
    comms.Topic("demo"), 
    func(ctx context.Context, data []byte) ([]byte, error) {
        return []byte("processed"), nil
    },
)

reply, err := send.Send(context.Background(), comms.Topic("demo"), []byte("some message"))
if err != nil {
    panic(err)
}

fmt.Println(string(reply)) // Prints processed
```
