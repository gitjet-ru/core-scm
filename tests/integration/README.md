# Integration tests

Integration tests can be run with PostgreSQL:
```shell
make test-pgsql
```

Make sure to perform a clean build before running tests:
```
make clean build
```

## Run pgsql integration tests
Setup a pgsql database inside docker
```
docker run -e "POSTGRES_DB=test" -e "POSTGRES_USER=postgres" -e "POSTGRES_PASSWORD=postgres" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```
Setup minio inside docker
```
docker run --rm -p 9000:9000 -e MINIO_ROOT_USER=123456 -e MINIO_ROOT_PASSWORD=12345678 --name minio bitnamilegacy/minio:2023.8.31
```
Start tests based on the database container
```
TEST_MINIO_ENDPOINT=localhost:9000 TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=postgres TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## Running individual tests

Example command to run GPG test:

```
make test-pgsql#GPG
```

## Setting timeouts for declaring long-tests and long-flushes

We appreciate that some testing machines may not be very powerful and
the default timeouts for declaring a slow test or a slow clean-up flush
may not be appropriate.

You can set the following environment variables:

```bash
GITEA_TEST_SLOW_RUN="10s" GITEA_TEST_SLOW_FLUSH="1s" make test-pgsql
```
