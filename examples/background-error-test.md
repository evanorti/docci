# Background Error Test

A background block can now assert what it prints while starting up, so this
file covers the failure instead: a background process that exits before the
output the page promises ever appears.

<!-- docci name="a service that dies on startup" background -->

```bash
echo "starting up"
exit 3
```

<!-- docci expect-output -->

```
listening on port <...>
```
