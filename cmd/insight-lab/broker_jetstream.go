//go:build jetstream

package main

// The optional NATS JetStream broker adapter (#135): only binaries built
// with `-tags jetstream` can use `-runtime distributed -broker nats://…`.
import _ "insight-lab/internal/execution/jetstream"
