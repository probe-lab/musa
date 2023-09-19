# 🍌 Musa

[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

[Musa](https://en.wikipedia.org/wiki/Musa_(genus)) is a lean bootstrapper for any libp2p-based network. It's called like that to capture the [🍌](https://en.wikipedia.org/wiki/Musa_(genus)) vibes [around here](https://github.com/ipfs/kubo/pull/8958).

## Table of Contents

- [Background](#background)
- [Install](#install)
- [Usage](#usage)
    - [Generator](#generator)
- [Badge](#badge)
- [Example Readmes](#example-readmes)
- [Related Efforts](#related-efforts)
- [Maintainers](#maintainers)
- [Contributing](#contributing)
- [License](#license)

## Background

The current bootstrappers run by [Protocol Labs](https://protocol.ai) are
instances of [Kubo](https://github.com/ipfs/kubo) and one other [written in Rust](https://github.com/libp2p/rust-libp2p/tree/master/misc/server) [[blog post](https://blog.ipfs.tech/2023-rust-libp2p-based-ipfs-bootstrap-node/)].
It is good to have implementation diversity in case of a regression that could
render the network unreachable. We have already [shipped a feature](https://github.com/ipfs/kubo/pull/8856)
that uses previously identified peers as backup bootstrap peers. However, it
will take time until peers will upgrade their Kubo installation and therefore
won't benefit from that feature for a while. With Musa we're adding to the effort
of diversifying the fleet of bootstrap peers.

Because of the way it is built, it can be configured to bootstrap into any
libp2p-based network. Further, we expect it to require minimal resources

## Usage

Just run

```sh
$ go run *.go
```

in the root directory of this repository.

### Tracing

To enable tracing, first start the [Jaeger](https://www.jaegertracing.io/) container:

```shell
docker run --rm --name jaeger -p 16686:16686 -p 4317:4317 jaegertracing/all-in-one:1.49
```

Then provide the `--trace-host` and `--trace-port` command line flags:

```sh
$ go run *.go --trace-host localhost --trace-port 4317
```

Traces will be available at [http://localhost:16686](http://localhost:16686).

## Configuration

There are plenty of configuration options. Just provide the `--help` command line flag

```text
NAME:
   musa - a lean bootstrapper process for any network

USAGE:
   musa [global options] command [command options] [arguments...]

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --host value          the network musa should bind on (default: "127.0.0.1") [$MUSA_HOST]
   --port value          the port on which musa should listen on (default: random) [$MUSA_PORT]
   --private-key value   base64 private key identity for the libp2p host (default: "127.0.0.1") [$MUSA_PRIVATE_KEY]
   --protocol value      the libp2p protocol for the DHT (default: "/ipfs/kad/1.0.0") [$MUSA_PROTOCOL]
   --metrics-host value  the network musa metrics should bind on [$MUSA_METRICS_HOST]
   --metrics-port value  the port on which musa metrics should listen on (default: 0) [$MUSA_METRICS_PORT]
   --trace-host value    the network musa trace should be pushed to [$MUSA_TRACE_HOST]
   --trace-port value    the grpc otlp port to which musa should push traces to (default: 0) [$MUSA_TRACE_PORT]
   --log-level value     the structured log level (default: 0) [$MUSA_LOG_LEVEL]
   --help, -h            show help
```

## Maintainers

[@ProbeLab](https://github.com/plprobelab).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/RichardLitt/standard-readme/issues/new) or submit PRs.

Standard Readme follows the [Contributor Covenant](http://contributor-covenant.org/version/1/3/0/) Code of Conduct.

## License

[MIT](LICENSE) © Protocol Labs