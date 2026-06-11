# Minimal template and instance

Register a one-node template, deploy it, create an instance, observe the node settle into `fresh`.

**Precondition:** a running rimsky deployment (stand one up from the published images — see the [operator guide](../../operator-guide.md)).

The `stub` executor here is the dockerized test stub executor (a test fixture, not a published image — see [`../../executors/stub/README.md`](../../executors/stub/README.md)). It returns a canned terminal `StreamClose{Success}` (`changed=false`, no attribute writeback) for every dispatch unconditionally, ignoring the request's `attributes` bag and `node_type`. (The `RIMSKY_EXECUTOR_STUB_MODE=1` env var is a separate mechanism on the `http-node` and verifier executors that short-circuits their network paths for testing; the test stub executor does not read it.)

## 1. The template

Save as `minimal.yml`:

```yaml
name: minimal
version: "1.0"
frame_resolution_mode: serial_queue
nodes:
  - type: hello
    executor: stub
    attributes:
      schema:
        type: object
        additionalProperties: true
```

## 2. Register and deploy

```sh
rimsky template register minimal.yml
# returns: template_hash=sha256-abc...def, tags=

rimsky template deploy sha256-abc...def
```

## 3. Create an instance

```sh
rimsky instance create sha256-abc...def
# returns: instance_id=01HZ..., template_hash=sha256-abc...def, node_count=1
```

## 4. Observe completion

```sh
curl http://localhost:8080/v1/instances/01HZ.../nodes
```

Expected output (after the stub executor returns):

```json
{
  "nodes": [
    {
      "node_type": "hello",
      "state": "fresh"
    }
  ]
}
```

## Verification

The `GET /v1/instances/{idOrKey}` response carries no frame-state field — frame state (`queued | running | completed | failed`) lives on `rimsky_frames` rows, not the instance projection. The settlement signal is the node state: the stub's terminal drives `hello` back to `fresh` (it ran, settled, and is idle awaiting the next invalidate).

```sh
curl -s http://localhost:8080/v1/instances/01HZ.../nodes \
  | jq '.nodes[] | select(.node_type == "hello") | .state'
```

Expected output: `"fresh"` (the node ran to terminal and settled).
