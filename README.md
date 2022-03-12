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

| Variable             | Default                   | Description                                      |
|----------------------|---------------------------|--------------------------------------------------|
| `AUTH_HEADER`        | `Authorization`           | Header where to find a token                     |
| `AUTH_HEADER_PREFIX` | <code>Bearer&nbsp;</code> | Prefix of header (note the space after `Bearer`) |
| `DEBUG`              | `false`                   | Enable debug log messages                        |
| `LISTEN`             | `:80`                     | Internal address and port to listen on           |
| `LOG_FORMAT`         |                           | Logging format: `json` `logfmt` else text mode   |
| `ROOT_DIR`           | `/uploads`                | Root directory for uploaded files                |
| `TOKEN_DELETE`       |                           | Token for DELETE method                          |
| `TOKEN_GET`          |                           | Token for GET method                             |
| `TOKEN_PUT`          |                           | Token for PUT method                             |
| `URL_BASE_PATH`      | `/`                       | Base path for URL                                |

### Upload

You can upload a file with `PUT` to the desired location.  
If a formfile at key `file` is present it gets saved else the request body is used as content.  
The filename will be ignored. Only the URL-path is important.  
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

You can delete a file or directory (recursively) with `DELETE`.  
Empty folders won't be deleted automatically.

```shell
curl -X DELETE example.com/test.txt
```

## Security

CORS is always enabled: `Access-Control-Allow-Origin: *`.

The server implements a very simple token auth per method.  
A token can be passed by the `token` query parameter or a header set with `AUTH_HEADER` and `AUTH_HEADER_PREFIX`.  
If token is provided both ways the query parameter has precedence.  
By default, the header is `Authorization` the token is `Bearer <token>`.

_Example:_  
To restrict access to PUT requests set the environment variable TOKEN_PUT to your desired token.  
Now you have to specify this token with your request parameters.

```shell
curl -X PUT -F "file=@my/file.jpg" example.com/some/dir/my-picture.jpg?token={your-token}
```

OR

```shell
curl -X PUT -F "file=@my/file.jpg" -H "Authorization: Bearer {your-token}" example.com/some/dir/my-picture.jpg
```
