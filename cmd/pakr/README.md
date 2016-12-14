## cmd/pakr

A helper utility for doing a command-line package resolves using the pakr library

### Usage

```
pakr -h

Usage of ./pakr:
  -index string
        Path to Index/Repo JSON file
  -reqs string
        Path to Requirements JSON file
```

"index" represents all of the available packages (their versions and requirements)

"req" represents the particular package constraints you want to resolve

Returns a json output indicating whether the solve succeded,
and either the package solution or an error messages explaining the failure.

See `test_index.json` and `test_requires.json` for format examples.

### Examples

```
$ ./pakr -index test_index.json -reqs test_requires.json

{
    "results": [
        {
            "product": "b",
            "version": "1.0.0"
        },
        {
            "product": "a",
            "version": "1.1.0"
        },
        {
            "product": "c",
            "version": "1.0.0"
        }
    ],
    "solved": true,
    "error": ""
}

$ ./pakr -index test_index.json -reqs test_requires_fail.json

{
    "results": null,
    "solved": false,
    "error": "
        The following requirements cannot be satisfied:
            b-1.0.0
            c-2.0.0

        Details:
        Package b-1.0.0 depends on one of (a-1.0.0, a-1.1.0)
        Package c-2.0.0 depends on one of (a-1.2.0)
        Package a-1.2.0 conflicts with (a-1.1.0)
        Package a-1.2.0 conflicts with (a-1.0.0)
        "
}
```