# 关于集成测试

使用如下 make 命令可以运行指定的集成测试：
```shell
make test-pgsql
```

在执行集成测试命令前请确保清理了之前的构建环境，清理命令如下：
```
make clean build
```

## 如何使用 pgsql 数据库进行集成测试
同上，首先在 docker 容器里部署一个 pgsql 数据库
```
docker run -e "POSTGRES_DB=test" -e "POSTGRES_USER=postgres" -e "POSTGRES_PASSWORD=postgres" -p 5432:5432 --rm --name pgsql postgres:latest #(just ctrl-c to stop db and clean the container)
```
在docker内设置minio
```
docker run --rm -p 9000:9000 -e MINIO_ROOT_USER=123456 -e MINIO_ROOT_PASSWORD=12345678 --name minio bitnamilegacy/minio:2023.8.31
```
之后便可以基于这个数据库进行集成测试
```
TEST_MINIO_ENDPOINT=localhost:9000 TEST_PGSQL_HOST=localhost:5432 TEST_PGSQL_DBNAME=postgres TEST_PGSQL_USERNAME=postgres TEST_PGSQL_PASSWORD=postgres make test-pgsql
```

## 如何进行自定义的集成测试

下面的示例展示了怎样在集成测试中只进行 GPG 测试：

pgsql 数据库:

```
make test-pgsql#GPG
```

