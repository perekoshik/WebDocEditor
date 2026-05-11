# LegalEdit

Веб-редактор юридических документов на Go + OnlyOffice Document Server.

## Запуск на сервере

### 1. Установка зависимостей

PostgreSQL и базовые утилиты:

```bash
sudo apt update
sudo apt install -y git make postgresql postgresql-contrib curl ca-certificates
```

Docker:

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER
newgrp docker
```

Go 1.22:

```bash
curl -LO https://go.dev/dl/go1.22.8.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.22.8.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

golang-migrate:

```bash
curl -L https://github.com/golang-migrate/migrate/releases/latest/download/migrate.linux-amd64.tar.gz | tar xz
sudo mv migrate /usr/local/bin/
```

Проверка:

```bash
go version
docker --version
psql --version
migrate -version
```

### 2. База данных

```bash
sudo -u postgres psql -c "CREATE USER legaledit WITH PASSWORD 'legaledit';"
sudo -u postgres psql -c "CREATE DATABASE legaledit OWNER legaledit;"
sudo -u postgres psql -c "GRANT ALL ON SCHEMA public TO legaledit;"
```

### 3. Клонирование репозитория

```bash
git clone https://github.com/perekoshik/WebDocEditor.git
cd WebDocEditor
```

### 4. Конфигурация

```bash
cp backend/.env.example backend/.env
mkdir -p ~/legaledit/docs
nano backend/.env
```

Содержимое `backend/.env` (заменить `<SERVER_IP>` на публичный IP или домен):

```
DATABASE_URL=postgres://legaledit:legaledit@127.0.0.1:5432/legaledit?sslmode=disable
FILES_DIR=/home/<user>/legaledit/docs
PUBLIC_API_URL=http://<SERVER_IP>:8080
INTERNAL_API_URL=http://host.docker.internal:8080
ONLYOFFICE_INTERNAL_URL=http://localhost:8082
ONLYOFFICE_PUBLIC_URL=http://<SERVER_IP>:8082
SERVER_ADDR=:8080
WEB_DIR=./web
```

- `DATABASE_URL` использует учётные данные из шага 2.
- `INTERNAL_API_URL` пишется именно как `host.docker.internal:8080`, потому что
  по этому адресу контейнер OnlyOffice ходит к Go-бэкенду.
- `PUBLIC_API_URL` и `ONLYOFFICE_PUBLIC_URL` должны быть доступны браузеру
  пользователя извне, поэтому здесь подставляется внешний IP сервера.

### 5. OnlyOffice Document Server

```bash
make -C backend onlyoffice-up
```

Через минуту проверьте что все запустилось

```bash
curl -sS -o /dev/null -w "%{http_code}\n" http://localhost:8082/
```

Ожидаемый ответ: `302`. Если `000` — подождите

### 6. Миграции

```bash
make -C backend migrate-up
```

### 7. Запуск сервера

```bash
make -C backend run
```

Лог покажет `level=INFO msg=listening addr=:8080`.

### 8. Открытие в браузере

`http://<SERVER_IP>:8080`


## Резервное копирование

```bash
make -C backend backup
```

Скрипт `backend/scripts/backup.sh` делает дамп PostgreSQL через `pg_dump`,
архивирует каталог `FILES_DIR` и складывает результат в `./backups/`. Файлы
старше 14 дней удаляются автоматически.

Для регулярного бэкапа можно добавить в cron:

```
0 3 * * * cd /home/<user>/WebDocEditor && make -C backend backup
```

## Остановка и обновление

```bash
make -C backend onlyoffice-down
docker stop onlyoffice || true
docker rm onlyoffice || true
git pull
make -C backend migrate-up
make -C backend onlyoffice-up
make -C backend run
```


## Полезные команды

| Команда                       |                                               |
| ----------------------------- | --------------------------------------------- |
| `make -C backend run`         | запуск приложения                             |
| `make -C backend build`       | компиляция в `backend/bin/server`             |
| `make -C backend migrate-up`  | применить миграции                            |
| `make -C backend migrate-down`| откатить одну миграцию                        |
| `make -C backend onlyoffice-up`| запустить контейнер OnlyOffice               |
| `make -C backend onlyoffice-down`| остановить и удалить контейнер             |
| `make -C backend backup`      | резервная копия БД и файлов                   |
