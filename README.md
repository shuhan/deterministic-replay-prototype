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
go run . replay --map serviceA=localhost:3000 --request [request-context]
```

to unfreeze only service A, it will trigger a debug request to serviceA with every other dependency frozen for the request.

Here [request-context] is the `X-Request-Context` header from the failing response

You can also unfreeze any of the other services, or multiple like

```
go run . replay --map serviceA=localhost:3000 serviceB=localhost:3001 --request [request-context]
```

### 4. Observers

Observers are a wrap around an state. For hidden states like config, cache access, data etc, a named typed observer can be used to freeze the variable or function response. In this example ServiceC has a hidden state, a counter. An observer is used to wrap the hit counter. Which mean's in debug even the stateful serviceC will behave determinstically. However to debug an observed state can be unfrozen like

```
go run . replay --map serviceC=localhost:3002 serviceC:HitCounter=pass --request [request-context]
```

Here the pass indicates pass through, unfreezing the state.

### 5. Regression

A regression can be run on any number of mapped services, this would verify that any changes made to fix a fialing request didn't unintentionally break something else.

```
go run . regress --map serviceB=localhost:3001
```

A request context can be passed to the regress to ask it to ommit it out of the regression test.

Additionally `--count` parameter would allow the number of succeeding requests to test against.
`--regress-dependency` allows running regression against any dependecy calls, where even if the final result passes the regression but dependecy requests change in shape are flagged.
`--allow-diversion` allows where dependency may have shifted from recorded response. This parameter doesn't request an argument. But passing `--allow-diversion no-flag` will stop diversions marked as dependency regression failure while allowing none diversed dependencies being checked.
