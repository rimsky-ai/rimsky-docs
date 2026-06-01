# Example guide (fixture)

The `OpenRequest` message starts a dispatch; the executor returns a
`StreamClose`. Background reading lives on GitHub and in the PostgreSQL docs —
prose proper nouns, not backticked, so not checked.

```proto
service Example {
  rpc Run(OpenRequest) returns (stream ExecuteEvent);
}
```
