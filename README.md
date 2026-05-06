# LDIFGenerator

Генератор больших LDIF-файлов для нагрузочного тестирования LDAP-сервера.

Что умеет:

- читает LDAP schema из `.ldif`, `.schema`, `.conf`;
- принимает один файл, несколько файлов или папку со схемами;
- парсит `attributeTypes` и `objectClasses`;
- учитывает `SUP`, `MUST`, `MAY`, aliases и folded LDIF values;
- генерирует users, privileged users, groups, computers, service accounts;
- пишет LDIF streaming-подходом, без хранения всех записей в памяти;
- валидирует записи перед записью в файл;
- умеет дробить LDIF и запускать конкурентный `ldapadd`.

## Быстрый старт

Сгенерировать LDIF по папке со схемами:

```bash
go run ./cmd/ldifgenerator \
  -schema /path/to/schema-dir \
  -config examples/config.json
```

Сгенерировать LDIF по нескольким файлам схемы:

```bash
go run ./cmd/ldifgenerator \
  -schema /path/to/core.ldif,/path/to/custom.ldif \
  -config examples/config.json
```

Результат будет записан в файл из `outputPath` в config JSON.

## Config

Минимальный рабочий config лежит здесь:

```bash
examples/config.json
```

Основные поля:

- `baseDN`: корневой DN, например `dc=example,dc=com`;
- `count`: сколько записей генерировать;
- `outputPath`: куда писать LDIF;
- `objectClasses`: objectClass для users/groups/computers/service accounts;
- `tree`: структура OU и проценты типов записей;
- `relationships`: группы, nested groups, managers;
- `optionalFillPercent`: процент заполнения MAY-атрибутов;
- `selectedAttributes`: ограничение MAY-атрибутов, если нужно.

`MUST`-атрибуты генерируются всегда. Если обязательный атрибут нельзя сгенерировать, генерация завершится ошибкой.

## UI

Запустить backend:

```bash
go run ./cmd/ldifgenerator-ui
```

Запустить frontend:

```bash
cd frontend
npm install
npm run dev
```

Открыть URL, который покажет Vite. Обычно это:

```text
http://localhost:5173
```

Собранный UI:

```bash
cd frontend
npm install
npm run build
cd ..
go run ./cmd/ldifgenerator-ui -static frontend/dist
```

По умолчанию backend слушает:

```text
http://127.0.0.1:8080
```

## Проверить схему

Посмотреть, сколько `attributeTypes` и `objectClasses` распарсилось:

```bash
go run ./cmd/schemaaudit /path/to/schema-dir
```

Можно передать несколько путей:

```bash
go run ./cmd/schemaaudit /path/to/core.ldif /path/to/custom.ldif
```

## Загрузка в LDAP

Обычный импорт через `ldapadd`:

```bash
ldapadd \
  -x \
  -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" \
  -w secret \
  -c \
  -f generated.ldif
```

Конкурентный импорт через `ldapbulkadd`:

```bash
go run ./cmd/ldapbulkadd \
  -file generated.ldif \
  -jobs 8 \
  -chunk-records 5000 \
  -- \
  -x \
  -H ldap://localhost:389 \
  -D "cn=admin,dc=example,dc=com" \
  -w secret \
  -c
```

`ldapbulkadd` делит LDIF на фазы:

- сначала containers/OU;
- потом обычные записи;
- потом groups с `member`.

Для nested groups лучше оставить:

```bash
-group-jobs 1
```

## Только разбить LDIF на чанки

```bash
go run ./cmd/ldapbulkadd \
  -file generated.ldif \
  -split-only \
  -workdir ./chunks \
  -chunk-records 5000
```

## Тесты

```bash
go test ./...
```

Frontend build:

```bash
cd frontend
npm install
npm run build
```

## Важные детали

- `userPassword` не генерируется.
- `NO-USER-MODIFICATION` атрибуты не генерируются.
- Атрибуты с неподдерживаемыми `SYNTAX`, `EQUALITY`, `ORDERING`, `SUBSTR` отключаются через `generator.DefaultAttributeSupportPolicy()`.
- Для включения отдельных конструкций в коде используйте `generator.NewWithAttributeSupportPolicy`.
- LDIF пишется потоково через buffered writer, поэтому подходит для 100k, 1M+ записей.
