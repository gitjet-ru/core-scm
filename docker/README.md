# Gitea - Docker

Dockerfile is found in the root of the repository.

Docker image can be found on [docker hub](https://hub.docker.com/r/gitea/gitea).

Documentation on using docker image can be found on [Gitea Docs site](https://docs.gitea.com/installation/install-with-docker-rootless).

## S3 via geesefs (GitJet local testing)

- Build target: `gitea-s3fs` (see [s3fs-local/README.md](s3fs-local/README.md)).
- Секреты: `cp .env.s3.example .env.s3`, заполнить; `.env.s3` в `.gitignore`.
- Compose merge: `docker compose -f docker-compose.yml -f docker-compose.s3fs-local.yml up -d --build gitea`

## Spilo / Patroni: `waiting for leader to bootstrap`, Spilo unhealthy

Patroni хранит состояние кластера **в etcd**, а не только в данных Postgres. Если сбросили том `spilo_data`, но **не** очистили ключи Patroni в etcd (или наоборот), узел может бесконечно писать `Lock owner: None` / `waiting for leader to bootstrap` (см. [zalando/spilo#690](https://github.com/zalando/spilo/issues/690)).

**Вариант 1 — снять кластер через `patronictl` (предпочтительно):**

```bash
# имя кластера = PATRONI_SCOPE из .env (по умолчанию gitjet-local)
docker exec -u postgres -it core-scm-spilo-1 patronictl list
docker exec -u postgres -it core-scm-spilo-1 patronictl remove gitjet-local
# подтвердить интерактивно; затем перезапуск spilo
docker compose restart spilo
```

**Вариант 2 — удалить ключи в etcd (без интерактива):**

```bash
docker compose stop spilo
docker exec core-scm-etcd-1 sh -c 'ETCDCTL_API=3 etcdctl del --prefix /service/gitjet-local/'
docker compose start spilo
```

Подставьте свой `PATRONI_SCOPE` вместо `gitjet-local` в пути и в `patronictl remove`.

**Вариант 3 — полный сброс локального стенда (данные БД пропадут):**

```bash
docker compose down
docker volume rm core-scm_etcd_data core-scm_spilo_data
docker compose up -d
```
