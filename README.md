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
| `LOG_FORMAT`         | _text mode_               | Logging format: `json` `logfmt` else _text mode_ |
| `ROOT_DIR`           | `/uploads`                | Root directory for uploaded files                |
| `TOKEN_DELETE`       |                           | Token for DELETE method                          |
| `TOKEN_GET`          |                           | Token for GET method                             |
| `TOKEN_PUT`          |                           | Token for PUT method                             |
| `URL_BASE_PATH`      | `/`                       | Base path for URL                                |
| `FORCE_DIGEST`       | `false`                   | Force the use of a digest for file uploads       |

### Upload

You can upload a file using the `PUT` method to the specified location.  
The server stores a _formfile_ under the key `file`.  
If the key is absent, the server uses the _request body_ as the file content.   
The filename is not considered, only the URL-path matters.  
If a file already exists at the location, it will be overwritten.

When uploading, a temporary file is created in the same directory as the destination file.  
If an error occurs, the temporary file is cleaned up. The destination file will not be created/updated.

_Examples:_

Create a file `test.txt` with content `Hello world`:

```shell
curl -X PUT -H "Content-Type: text/plain" -d "Hello world" example.com/test.txt
```

Upload `./my/file.jpg` as `/some/dir/my-picture.jpg`:

```shell
curl -X PUT -F "file=@my/file.jpg" example.com/some/dir/my-picture.jpg
```

You can use the [Digest header](https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Digest) to verify file
integrity.  
Supported hashing functions are `MD5` and `SHA-256`.   
All provided digest values are evaluated.

```shell
curl -X PUT -H "Content-Type: text/plain" -H "Digest: md5=PiWWCnnbxptnTNTsZ6csYg==,sha-256=ZOyIygCyaOW6GjVnihtTFtIS9PNmskdyMlNKiuyjfzw=" -d "Hello world" example.com/test.txt
```

### Delete

You can delete a file or directory (recursively) with `DELETE`.  
Empty folders are automatically removed.

```shell
curl -X DELETE example.com/test.txt
```

## Security

> Notice: CORS is always enabled: `Access-Control-Allow-Origin: *`.

The server implements a very simple token auth per method.  
You can pass the token either as a `token` query parameter or via a header defined by `AUTH_HEADER`
and `AUTH_HEADER_PREFIX`.  
If a token is provided both ways, the query parameter takes precedence.  
By default, the header is `Authorization` the token is format `Bearer <token>`.

_Example:_  
To restrict access to PUT requests, set the environment variable `TOKEN_PUT` to your desired token.  
Now you have to specify this token with your request parameters.

```shell
curl -X PUT -F "file=@my/file.jpg" example.com/some/dir/my-picture.jpg?token={your-token}
```

OR

```shell
curl -X PUT -F "file=@my/file.jpg" -H "Authorization: Bearer {your-token}" example.com/some/dir/my-picture.jpg
```
