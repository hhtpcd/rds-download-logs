# rds-download-logs

Download/tail logs from AWS RDS database instances. Uses AWS credentials in your environment

```
AWS_ACCESS_KEY_ID=
AWS_SECRET_ACCESS_KEY=
```

## Usage

```
$ ./rds-download-logs --help
Usage:
  rds-download-logs [OPTIONS]

Application Options:
  -d, --database= Identifier of the database instance
  -r, --region=   Amazon region (default: eu-west-1)
  -o, --output=   Location for saving the log (default: .)
  -f, --follow    Follow a log named in -l flag
  -l, --log=      Specify a log when used in conjunction with -s/-l
  -s, --save      Choose to download a logfile
  -p, --print     Print log file names to stdout

Help Options:
  -h, --help      Show this help message
```

### Examples

```
# Tail a database log
rds-download-logs --database=most-excellent-database --follow --region=eu-west-1

# Download an entire database log file
rds-download-logs --database=most-execellent-database --save --output=/backup/location/logfile.log --region=eu-west-1

# Print all available log names that match prefix
rds-download-logs --database=most-excellent-database --print --log=slowquery --region=eu-west-1
```

