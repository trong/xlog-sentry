# XLog to Sentry Output

[![godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/trong/xlog-sentry)

xlog-sentry is an xlog to [Sentry](https://getsentry.com) output for [github.com/rs/xlog](https://github.com/rs/xlog).

## Install

    go get github.com/trong/xlog-sentry

## Usage

```go
o := xlogsentry.NewSentryOutput(YOUR_DSN, nil)
o.Timeout = 300 * time.Millisecond
o.StacktraceConfiguration.Enable = true

l := xlog.New(xlog.Config{
	Output: o,
	Fields: xlog.F{
	    "role": "my-service",
	},
})

l.Errorf("What: %s", "happens?")
```

## Licenses

All source code is licensed under the [MIT License](https://raw.github.com/trong/xlog-sentry/master/LICENSE.md).