# CockroachDB Observability Service

This directory contains the Observability Service library (`obslib`). A
CockroachDB node can embed it to ingest the events it exports; see
`pkg/server/server_obs_service.go`. The standalone binary and its DB Console
proxy are gone.

## Event ingestion

The Obs Service ingests events using the
[OTLP](https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/otlp.md)
Logs [gRPC
service](https://github.com/open-telemetry/opentelemetry-proto/blob/2119dc9affc4c246f9227fa5411765b81bc91f87/opentelemetry/proto/collector/logs/v1/logs_service.proto).
CRDB exports events using a gRPC client. The events are records are grouped into
[`ResourceLogs`](https://github.com/open-telemetry/opentelemetry-proto/blob/200ccff768a29f8bd431e0a4a463da7ed58be557/opentelemetry/proto/logs/v1/logs.proto)
and,within that, into
[`ScopeLogs`](https://github.com/open-telemetry/opentelemetry-proto/blob/200ccff768a29f8bd431e0a4a463da7ed58be557/opentelemetry/proto/logs/v1/logs.proto#L64).
A resource identifies the cluster/node/tenant that is emitting the respective
events. A scope identifies the type of event; events of different types get
routed to different processing pipelines, based on this event type. Events of 
unrecognized types are dropped. Currently, a single event type is supported: `"eventlog"`.
The log records carry attributes and a JSON payload representing the event.

## Licensing

The Observability Service is licensed as Apache 2.0.
