# hotreload [![Go](https://github.com/ryicoh/hotreload/actions/workflows/go.yml/badge.svg)](https://github.com/ryicoh/hotreload/actions/workflows/go.yml)

`hotreload` watch files and executes a command.

# Installation

```bash
$ go install  github.com/ryicoh/hotreload@latest
```



# Usage

```bash
hotreload -include="**/*.go" -verbose -cmd="go run ./main.go"
```


# Related works

* https://github.com/cosmtrek/air
* https://github.com/oxequa/realize

