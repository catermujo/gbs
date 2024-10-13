# gbs


<!--[![codecov](https://codecov.io/gh/isinyaaa/gbs/graph/badge.svg?token=DJU7YXWN05)](https://codecov.io/gh/isinyaaa/gbs)-->
[![Go Test](https://github.com/isinyaaa/gbs/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/isinyaaa/gbs/actions/workflows/go.yml)
[![license](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![go-version](https://img.shields.io/badge/go-%3E%3D1.20-30dff3?style=flat-square&logo=go)](https://github.com/isinyaaa/gbs)

gbs is short for Go binary socket.
This is a WIP for a netcode to support a distributed trading environment.

It has the following goals:

- Efficient cache usage
- Very little processing overhead
- UDP-compatibility (currently a stretch goal)

This is based on [lxzan/gws](https://github.com/lxzan/gws), which is an awesome WebSocket library.
