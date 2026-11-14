# OpenAPI dan Halaman `/docs`

## 1. Keputusan

- OpenAPI 3.1 YAML menjadi source of truth untuk Go API.
- Setiap operation memiliki `operationId`, tag domain, security scheme, request,
  response sukses, error response, pagination, dan idempotency rule.
- Spec divalidasi di CI dan dipakai untuk generated DTO/client atau validator.
- Swagger/Scalar/Redoc UI disajikan pada `https://api.algohub.web.id/docs`;
  endpoint JSON/YAML spec berada di `/docs/openapi.json` dan `/docs/openapi.yaml`.
- Halaman docs tidak menampilkan secret, internal hostname, atau contoh data nyata.

## 2. Struktur kontrak

```text
docs/openapi/openapi.yaml
docs/openapi/components/schemas.yaml
docs/openapi/components/errors.yaml
docs/openapi/paths/auth.yaml
docs/openapi/paths/admin.yaml
docs/openapi/paths/practicum.yaml
docs/openapi/paths/learning.yaml
docs/openapi/paths/academic.yaml
```

Path di spec memakai `/v1/...`; Vercel rewrite dan host tidak boleh mengubah
operationId atau schema response.

## 3. Error dan compatibility

Error code stabil minimal: `VALIDATION_ERROR`, `UNAUTHENTICATED`, `FORBIDDEN`,
`NOT_FOUND`, `CONFLICT`, `RATE_LIMITED`, `RUNNER_TIMEOUT`, `DEPENDENCY_FAILURE`,
`INTERNAL_ERROR`. Endpoint/RPC legacy diberi adapter, `Deprecation` header, tanggal
akhir, dan contract test sampai dihapus.

## 4. Test OpenAPI

- lint spec;
- validate example request/response;
- generated code diff check;
- handler contract test untuk status/error/header;
- negative test auth, permission, pagination, malformed JSON, dan idempotency;
- browser smoke membuka `/docs` dan mencoba endpoint yang aman/read-only.
