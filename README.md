delete-s3-version: a go library for deleting older versions of S3 files
=======================================================================

This library provides functionality to delete versions of S3 files. It finds the buckets that have
[versioning enabled](https://docs.aws.amazon.com/AmazonS3/latest/user-guide/enable-versioning.html)
and looks for files with multiple versions. It will keep the most recent versions and delete the
older versions and [delete markers](https://docs.aws.amazon.com/AmazonS3/latest/dev/DeleteMarker.html).

### Installation

```bash
go get -u github.com/croman/delete-s3-versions
```

### Command Line Flags

`delete-s3-versions` accepts these command line parameters:

```
Usage:
  delete-s3-versions [OPTIONS]

Application Options:
  -r, --s3-region=      The S3 region (default: eu-west-1)
  -s, --s3-disable-ssl= Disable SSL with S3 (default: false, used when having a local S3 stack)
  -e, --s3-endpoint=    S3 endpoint (used when having a local S3 stack)
  -b, --bucket=         (Required) The bucket name to check. Use '*' to check all buckets
  -p, --prefix=         The bucket prefix path (useful with big buckets to reduce running time)
  -n, --count=          How many versions to keep (keep the latest n versions or delete markers)
      --confirm         By default it prints details for the files to be deleted, enabling this flag leads to deleting S3 file versions

Help Options:
  -h, --help            Show this help message
```

Examples:

- Print all buckets summary for the versions about to delete, while keeping the latest 4 changes unchanged.

```bash
delete-s3-versions -r "us-east-1" --bucket "*" -n 4
```

- Permanently delete older S3 file versions from `my-bucket`. The newest 4 changes (versions or delete markers) remain unchanged.

```bash
delete-s3-versions -r "us-east-1" --bucket "my-bucket" -n 4 --confirm
```

Output example:

```
Check if bucket exists ...
Found these buckets [my-bucket my-other-bucket]
Found these buckets with versioning enabled [my-bucket]
Get file versions for my-bucket
  Got 1000 versions for page 1
  Got 131 versions for page 2
Summary: 1131 file versions for 10 files
Versions to delete for path/to/file.txt (count = 3):
  8tDv5iNX_I4E3G (400 MB)
  or807WVokOaUjBUe (500 MB)
  NibFS5hUrNR18FJmW5Dhl (0 B)
Versions to delete for path/to/another-file.txt (count = 2):
  8tDv5iNX_I4322 (100 MB)
  NibFS5JmW5Dhl (0 B)
Total space recovered for my-bucket: 1 GB
Total versions to delete for my-bucket: 5
```

## License

MIT
