# Deterministic Replay Prototype

This is an experimental prototype exploring deterministic replay for distributed systems.

The goal is to reduce the time spent reproducing production failures by capturing dependency state and enabling selective replay.

- This is not production-ready software.
- It was built to validate the architectural approach and developer experience.

## Require

```
go 1.24.0
```

## Run

Run the following in sequence

### 1. Backend Runtime

`cd backend-runtime && go run .` it shall start on port `8080`

### 2. Examples

For each of the three services in `example` directory, run them in debug mode. They will listen to port `3000`, `3001` and `3002`

Now you can use postman to request serviceA http://localhost:3000/boost?name=xx

try hitting the URL a few time until you notice the 500 error. We would need the `X-Request-Context` header from the failing response to debug this specific request.

If you send one more request, you will notice that it's working again, essentially creating a flecky bug.

### 3. Debug

Nvaigate to `cli` run

```
go run . replay [request-context] --map serviceA=localhost:3000
```

to unfreeze only service A, it will trigger a debug request to serviceA with every other dependency frozen for the request.

Here [request-context] is the `X-Request-Context` header from the failing response

You can also unfreeze any of the other services, or multiple like

```
go run . replay [request-context] --map serviceA=localhost:3000 serviceB=localhost:3001
```

### 4. Observers

Observers are a wrap around an state. For hidden states like config, cache access, data etc, a named typed observer can be used to freeze the variable or function response. In this example ServiceC has a hidden state, a counter. An observer is used to wrap the hit counter. Which mean's in debug even the stateful serviceC will behave determinstically. However to debug an observed state can be unfrozen like

```
go run . replay [request-context] --map serviceC=localhost:3002 serviceC:HitCounter=pass
```

Here the pass indicates pass through, unfreezing the state.
