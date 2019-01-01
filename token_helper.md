# Token helper

Token helper applies when `gitlab` auth method is used.

By default Rhythm CLI can use access token stored on disk in the `~/.rhythm-token` file. This functionality can be changed via the use of token helper. A token helper is an external program that Rhythm calls to retrieve saved token. The token helper could be a very simple script or a more complex program depending on your needs. The interface to the external token helper is extremely simple.

## Configuration

To configure a token helper, set `RHYTHM_TOKEN_HELPER` to absolute path to the token helper program. Program must be executable.

## Developing a Token Helper
The interface to a token helper is extremely simple: the script is passed with one argument that could be `read`, `update` or `delete`. If the argument is `read`, the script should do whatever work it needs to do to retrieve the stored token and then print the token to `STDOUT`. If the argument is `update`, Rhythm is asking to store the token. Finally, if the argument is `delete`, program should erase the stored token.

If program succeeds, it should exit with status code 0. If it encounters an issue that prevents it from working, it should exit with some other status code. You should write a user-friendly error message to `STDERR`. You should never write anything other than the token to `STDOUT`, as Rhythm assumes whatever it gets on `STDOUT` is the token.

## Example Token Helper
This is an example token helper that stores and retrives token in `~/.rhythm-token` in a similar way as default Rhythm CLI behaviour described above.

```bash
#!/usr/bin/env bash

set -e

TOKEN_FILE="${HOME}/.rhythm-token"

if [ "x$1" = "xread" ]; then
  cat "${TOKEN_FILE}"
elif [ "x$1" = "xupdate" ]; then
  cat > "${TOKEN_FILE}"
elif [ "x$1" = "xdelete" ]; then
  rm "${TOKEN_FILE}"
else
  exit 1
fi
exit 0
```
