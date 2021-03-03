# very-simple-upload-server

This is a _very_ simple upload server supporting GET, PUT and DELETE.

## Tags

* `latest`
* `alpine`
* `{version}`
* `{version}-alpine`

## Usage

Run with persistence on port 80:

```shell
docker run -v my_volume:/uploads -p 80:80 hallotschuess/very-simple-upload-server
```

Change base URL-path:

```shell
docker run -v my_volume:/uploads -p 80:80 -e URL_BASE_PATH=/my-base-path/ hallotschuess/very-simple-upload-server
```

### Environment Variables

| Variable        | Default    | Description                                     |
|-----------------|------------|-------------------------------------------------|
| `DEBUG`         | `false`    | Enable debug log messages                       |
| `LISTEN`        | `:80`      | Internal address and port to listen on          |
| `LOG_FORMAT`    |            | Logging format: `json` `logfmt` else text mode  |
| `ROOT_DIR`      | `/uploads` | Root directory for uploaded files               |
| `URL_BASE_PATH` | `/`        | Base path for URL                               |

### Upload

You can upload a file with `PUT` to the desired location.\
If a formfile at key `file` is present it gets saved else the request body is used as content.\
The filename will be ignored. Only the URL-path is important.\
Any existing file gets overwritten.

Create a file `test.txt` with content `Hello world`:

```shell
curl -X PUT -H "Content-Type: text/plain" -d "Hello world" example.com/test.txt
```

Upload `./my/file.jpg` as `/some/dir/my-picture.jpg`:

```shell
curl -X PUT -F "file=@my/file.jpg" example.com/some/dir/my-picture.jpg
```

### Delete

You can delete a file or directory (recursively) with `DELETE`.\
Empty folders won't be deleted automatically.

```shell
curl -X DELETE example.com/test.txt
```

## Security

The server **does not** implement **any** kind of security. \
Everybody can upload, download and delete any file as they please. \
CORS is always enabled: `Access-Control-Allow-Origin: *`
