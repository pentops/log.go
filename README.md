Log
===

An attempt to solve the 'company standard log library' problem with a very
simple logging interface for go built around context values.

Why
---

I have worked in a few large Go teams. Each one of them had a `pkg` or `std`
github repo containing the team's standard for basic go tasks like logging,
tracing, http (circuit breaking), sqs wrappers etc etc. As a rule, I always
encourage these to be individual independent components, which could be
open-source and reusable for many teams. The problem has always been how these
standard components log without tightly coupling them to the logging library
itself.

The popular logging frameworks for go cannot be described purely as an
interface. e.g., Logrus uses `*logrus.Fields`


Use Cases
---------

### Add context information

Including log information from the context. e.g. trace headers.

Assuming a middleware stack, include this as or in a middleware after the
context value is created:

```
ctx = log.WithField(ctx, "trace", trace.FromContext(ctx))

```

Later calls to `log.Info(ctx, "hello")` will include the trace header

### Library Use

Libraries shouldn't log. But they need to. But they shouldn't.

Solution: Include a minimal logging interface in your library, which defaults
to do nothing, but can be wired in to a real logger.

```
type SQSWorker struct {
	Logger interface{
		Info(context.Context, msg)
		WithFields(context.Context, map[string]interface{}) context.Context
	}
}
```




The logger interface is designed to be copied (not referenced) in libraries.
