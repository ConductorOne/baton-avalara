# Avalara mock server

`cmd/test-server` is a stateful mock of the Avalara API. Use it to test the connector in service mode.

Build and start it with `make mock-server`, or follow these steps.

1. Start the mock with 2500 generated users:
   `./test-server --addr 127.0.0.1:18080 --users 2500`
2. Start the connector against the mock:
   `BATON_C1_API_HOST=<host:port> ./baton-avalara --client-id <id> --client-secret <secret> --username testuser --password testpass --base-url http://localhost:18080`
3. Sync in C1. Compare the result with the fixture data and with `curl localhost:18080/_mock/stats`.
4. Change users through the admin API, then sync again:
   `curl -X PATCH localhost:18080/_mock/users/12345 -d '{"securityRoleId":"SystemAdmin","isActive":false}'`
   Other routes: `POST /_mock/users`, `DELETE /_mock/users/{id}`, `GET`/`PUT /_mock/state`, `POST /_mock/reset`.
5. Inject a 429, then a 500, and sync again:
   `curl -X POST localhost:18080/_mock/faults -d '{"path":"/api/v2/users","status":429,"retryAfter":2,"count":3}'`
   `curl -X POST localhost:18080/_mock/faults -d '{"path":"/api/v2/users","status":500,"count":1}'`
   Remove all faults with `DELETE /_mock/faults`.
6. Start the connector with a wrong password. Check the validation error in C1. The ping call returns HTTP 200 with `authenticated: false`, like the real API.

## Flags

| Flag | Default | Meaning |
| :--- | :--- | :--- |
| `--addr` | `:8080` | Listen address |
| `--username`, `--password` | `testuser`, `testpass` | Accepted Basic auth |
| `--fixture` | none | JSON file with `roles` and `users`. Try `cmd/test-server/testdata/fixture.json` |
| `--users` | 0 | Generate N users across all roles |
| `--latency` | 0 | Delay for each API response |
| `--ping-401` | false | Return 401 for wrong credentials on ping |
| `--admin-allow-remote` | false | Allow non-loopback clients on `/_mock/` |

`BASE_URL` sets the `@nextLink` origin. If it is not set, the mock uses the request scheme and host.
