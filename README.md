# OT Go Driver [![GoDoc](https://godoc.org/github.com/itcomusic/ot?status.png)](https://godoc.org/github.com/itcomusic/ot) [![Coverage Status](https://coveralls.io/repos/github/itcomusic/ot/badge.svg)](https://coveralls.io/github/itcomusic/ot)
The OpenText driver for Go

---

- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Examples](https://github.com/itcomusic/ot/tree/master/example)
- [License](#license)
---

## Requirements
- Go 1.12 or higher

- OpenText 16.x

## Installation

```bash
go get -u github.com/itcomusic/ot
```

### Usage
To get started with the driver, import the `ot` package, create a ot.Endpoint and set user to future authentication to your running OpenText server:
```go
ss := ot.NewEndpoint("127.0.0.1").User("test", "test")
```
To do this in a single step, you can use the Call function:
```go
var res map[string]interface{}
ctx, _ = context.WithTimeout(context.Background(), 30*time.Second)
if err := ss.Call(ctx, "service.method", oscript.M{"argName", 1}, &res); err != nil {
    log.Fatal(err)
}
```

A context does not stop execution of a request in the opentext, it closes only socket.

### Token

```go
token, err := ss.GetToken(context.Background(), "test", "test")
if err != nil {
    log.Fatal(err)
}
sst := ot.NewEndpoint("127.0.0.1").Token(token)
```



### Attributes

To set many attributes by types at once.

```go
if err := cat.Set(
    ot.AttrInt("attr1", 1)
    ot.AttrString("attr2", "hello")
    ot.AttrBool("attr3", true)
    ot.AttrTime("attr4", time.Now())); err != nil {
    log.Fatal(err)
}
```

To get attribute with explicit type.

```go
var attr1 int
if err := cat.Int("attr1", &attr1); err != nil {
    log.Fatal(err)
}
```

## License
The OT Go driver is licensed under the [MIT](LICENSE)
