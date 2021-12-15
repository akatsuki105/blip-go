# blip-go

[![Go Reference](https://pkg.go.dev/badge/github.com/pokemium/blip-go.svg)](https://pkg.go.dev/github.com/pokemium/blip-go)

Go binding for [blip-buf](https://code.google.com/archive/p/blip-buf).

This package does **not** depend on cgo.

## What is blip-buf?

The explanation of [blip_buf-rs](http://mvdnes.github.io/rust-docs/blip_buf-rs/blip_buf/index.html) is easy to understand and is quoted here.

> blip_buf is a small waveform synthesis library meant for use in classic video game sound chip emulation. It greatly simplifies sound chip emulation code by handling all the details of resampling. The emulator merely sets the input clock rate and output sample rate, adds waveforms by specifying the clock times where their amplitude changes, then reads the resulting output samples.

## Usage

```sh
go get github.com/pokemium/blip-go
```

## Credits

- [blip-buf](https://code.google.com/archive/p/blip-buf), which is copyright © 2003 – 2009 Shay Green and used under a Lesser GNU Public License.
