# Magicbox

Magicbox is a lightweight server app designed to aggressively cache and serve static sites from S3-compatible object storage engines.

It's designed to sit behind a reverse proxy, such as Caddy, and can support multiple site. Resources are cached for 30 days and will hold up to 1GB in-memory before evicting old entries.

## Bucket layout

Magicbox will only work within a dedicated bucket. At the root level, one directory corresponds to one "resource".

```
.
├── publicityGenerator/
│   ├── index.html
│   ├── favicon.ico
│   └── build/
│       └── ....
└── backseat/
    └── ...
```

Magicbox will serve a site on its main socket based on the `X-Magicbox-Resource` header, that should correspond to a directory name in the base of the bucket that matches the regex `^[a-zA-Z\d\-_+.]+$`

## Sample Caddy configuration

```
publicityGenerator.domain.xyz {
    Header -X-Magicbox-Resource # strip control header from incoming requests
    reverse_proxy magicbox:8080 {
        header_up X-Magicbox-Resource publicityGenerator
    }
}
```

## Cache invalidation

A resource's cache can be invalidated using the admin endpoint, `PUT /invalidate/{resource}`, eg:

```
curl -D - -X PUT -H "Authorization: Bearer <admin token>" <Magicbox admin address>/invalidate/publicityGenerator
```

## Configuration options

* General config:
  * `MAGICBOX_HTTP_ADDRESS`: address to use for the main socket, default `127.0.0.1:8080`
  * `MAGICBOX_MAX_CACHE_MB`: maximum cache size in megabytes, default 1024MB
* S3 bucket configuration:
  * `MAGICBOX_S3_BUCKET_NAME`: **required**
  * `MAGICBOX_S3_CREDENTIAL_ID`: **required**
  * `MAGICBOX_S3_CREDENTIAL_SECRET`: **required**
  * `MAGICBOX_S3_ENDPOINT`: **required**
  * `MAGICBOX_S3_REGION`: **required**
* Admin API config:
  * `MAGICBOX_ADMIN_ENABLED`: boolean, enable or disable the entire admin API, default `true`
  * `MAGICBOX_ADMIN_TOKEN`: when set, requires the `Authorization` header to be set with the value of the variable as a bearer token
  * `MAGICBOX_ADMIN_HTTP_ADDRESS`: default `127.0.0.1:8081`
