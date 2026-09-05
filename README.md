# zvec-go

A **lightweight Go client and HTTP service** for [zvec](https://github.com/alibaba/zvec),
a high-performance vector database.

zvec-go ships two independent pieces:

1. **`zvec`** — a dependency-free, pure-Go client library (stdlib only). It models
   collections/tables, documents (scalar fields + vector fields), and vector
   similarity search, with simple file-backed persistence.
2. **`zvec-httpd`** — a lightweight HTTP service (stdlib only) built on the client.
   It supports **multiple collections/tables**, **HTTP Basic auth** driven by a JSON
   config file, a full REST API, and an **embedded web console** served at `/`.

> There is also an **optional, experimental CGO binding** (`cgo/`, build-tag gated)
> that links against the real zvec C++ core. It is *not* required by either piece
> above and is off by default.

## Layout

```
.
├── zvec.go, collection.go, schema.go, types.go   # pure-Go client (package zvec)
├── server/                                        # the HTTP service (package server)
│   ├── server.go, config.go, auth.go, manager.go
│   ├── handlers.go, types.go, web.go
│   └── web/index.html                            # embedded web console
├── cmd/zvec-httpd/                               # the runnable service binary
├── cgo/                                          # optional experimental CGO binding
├── config.example.json                           # sample service config
├── lib/                                          # prebuilt zvec C++ static libraries
└── zvec/                                         # alibaba/zvec C++ submodule
```

---

## 1. The pure-Go client (`zvec`)

```go
import zvec "github.com/oliveagle/zvec-go"

zvec.Init(nil) // uses default config

schema := zvec.NewCollectionSchema("movies")
schema.AddVectorField(
    zvec.NewVectorSchema("emb", zvec.DataTypeVectorFP32, 128).WithMetricType(zvec.MetricTypeCOSINE),
)

coll, _ := zvec.CreateAndOpen("./data/movies", schema, nil)
coll.Upsert(zvec.NewDocument("m1").
    SetField("title", "Inception").
    SetVector("emb", vec))

results, _ := coll.Query(zvec.NewVectorQueryByVector("emb", queryVec).WithTopK(10), 10, "", false, nil)
```

Collections are persisted to disk (`<path>/collection.json` + `<path>/docs/*.json`) and
re-opened transparently by `zvec.Open`, so data survives restarts.

---

## 2. The HTTP service (`zvec-httpd`)

### Configuration

The service is configured by a **JSON file**. Copy `config.example.json` to
`config.json` and edit it:

```json
{
  "server": {
    "addr": "0.0.0.0:8080",
    "read_timeout_seconds": 30,
    "write_timeout_seconds": 30,
    "shutdown_timeout_seconds": 10
  },
  "storage": {
    "data_dir": "./data"
  },
  "auth": {
    "enabled": true,
    "users": [
      { "username": "admin",  "password": "sha256:8c6976e5b5410415bde908bd4dee15dfb167a9c873fc4bb8a81f6f2ab448a918" },
      { "username": "viewer", "password": "sha256:d35ca5051b82ffc326a3b0b6574a9a3161dee16b9478a199ee39cd803ce5b799", "readonly": true }
    ]
  }
}
```

- **`auth.enabled`** — when `true`, all `/api/v1/*` routes require HTTP Basic auth.
- **`auth.users[].password`** — either a plain string (a startup warning is emitted)
  or a **`sha256:<64 hex>`** hash. Hashes are compared in constant time.
- **`auth.users[].readonly`** — a readonly user can read and query, but any
  create/modify/delete returns `403 Forbidden`.
- **`storage.data_dir`** — each collection is a subdirectory here; collections are
  re-opened automatically on startup.

### Running

```bash
go build -o zvec-httpd ./cmd/zvec-httpd
./zvec-httpd -config config.json
# optional: -log-level debug|info|warn|error
```

Generate a password hash for your config:

```bash
./zvec-httpd -hash-password "mysecret"
# -> sha256:...
```

### Web console

Open **`http://<host>:<port>/`** (or `/ui`) in a browser. Enter a username and
password, then you can:

- **Create / delete** collections or tables (define scalar and vector fields).
- **Add / list / delete** documents (the form is generated from the collection schema).
- **Run vector queries** (pick a vector field, enter a comma-separated vector, set
  top-k) and view ranked results with scores.

The console is fully self-contained (single embedded HTML file, vanilla JS, no build
step) and talks to the same-origin `/api/v1` endpoints, so Basic auth is seamless.

### REST API

All `/api/v1/*` routes require Basic auth (when enabled). `GET /healthz` is public.

| Method | Path | Description |
|--------|------|-------------|
| `GET`    | `/healthz` | Service health (public) |
| `GET`    | `/api/v1/collections` | List collections (name, doc count, size, schema) |
| `POST`   | `/api/v1/collections` | Create a collection/table |
| `GET`    | `/api/v1/collections/{name}` | Collection details + stats |
| `DELETE` | `/api/v1/collections/{name}` | Drop a collection |
| `GET`    | `/api/v1/collections/{name}/documents` | List documents (`?limit=&offset=`) |
| `POST`   | `/api/v1/collections/{name}/documents` | Upsert one (`document`) or batch (`documents`) |
| `GET`    | `/api/v1/collections/{name}/documents/{id}` | Get one document |
| `DELETE` | `/api/v1/collections/{name}/documents/{id}` | Delete one document |
| `POST`   | `/api/v1/collections/{name}/query` | Vector similarity search |

#### Examples

```bash
B=http://127.0.0.1:8080

# create a collection (table + vector)
curl -u admin:admin -H 'Content-Type: application/json' -X POST $B/api/v1/collections -d '{
  "name": "movies",
  "description": "demo",
  "fields": [ {"name":"title","data_type":"STRING"}, {"name":"year","data_type":"INT64"} ],
  "vector_fields": [ {"name":"emb","data_type":"VECTOR_FP32","dimension":128,"metric_type":"COSINE"} ]
}'

# upsert documents
curl -u admin:admin -H 'Content-Type: application/json' -X POST $B/api/v1/collections/movies/documents -d '{
  "documents": [
    {"id":"m1","fields":{"title":"Inception","year":2010},"vectors":{"emb":[ ... 128 floats ... ]}}
  ]
}'

# vector query
curl -u admin:admin -H 'Content-Type: application/json' -X POST $B/api/v1/collections/movies/query -d '{
  "field": "emb", "vector": [ ... 128 floats ... ], "top_k": 10, "include_vector": false
}'
```

---

## 3. Building & testing

```bash
go build ./...
go test  ./...          # client + service tests
go vet   ./...
```

> **Note on the `zvec/` C++ submodule:** `zvec/` is the [alibaba/zvec](https://github.com/alibaba/zvec)
> C++ project (currently pinned to **v0.7.0**). It is a separate Go module boundary
> (`zvec/go.mod`) so the parent `./...` build does not descend into its vendored
> third-party Go sources. If you re-clone the submodule and a nested `go.mod` is
> missing, all the targets above still build; only a full `go build ./...` from the
> root needs `zvec/go.mod` present.

## 4. zvec C++ core & the experimental CGO binding

- The zvec C++ submodule is pinned to **v0.7.0**.
- Prebuilt static libraries live in `lib/` (see `lib/README.md`). They are rebuilt and
  committed automatically by the `build-zvec-static-libraries` GitHub Actions workflow
  whenever the submodule or workflow changes.
- The `cgo/` package is an **optional, experimental** binding to the C++ core. It is
  excluded from normal builds. To build it against the C++ core:

  ```bash
  CGO_ENABLED=1 go build -tags cgo,zvec_cgo ./cgo/
  ```

  It requires the prebuilt static libraries in `lib/` that match the pinned zvec version.
  The binding currently exposes a small linkage-probing surface; the full
  Collection/Doc/Query surface is a follow-up.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
