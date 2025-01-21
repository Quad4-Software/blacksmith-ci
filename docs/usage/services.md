<!-- markdownlint-disable MD041 -->
Crow provides a `services:` section in the pipeline config file for "service" containers.
These are static daemon services like databases or APIs which are then used/consumed by other containers within the pipeline.

To illustrate possible use cases, the below examples uses a "database" and a "cache" service.

## Hostnames

Services are accessed via DNS using their assigned hostnames.
In the example below, the MySQL service is assigned the hostname `database` and is subsequently available at `database:3306`.

```yaml hl_lines="9 12"
steps:
  - name: build
    image: golang
    commands:
      - go build
      - go test

services:
  - name: database
    image: mysql

  - name: cache
    image: redis
```

## Ports and Protocol

Defining ports and their protocol is supported:

```yaml
services:
  - name: database
    image: mysql
    ports:
      - 3306

  - name: wireguard
    image: wg
    ports:
      - 51820/udp
```

## Configuration

Service containers allow exposing environment variables to customize their startup.

```yaml hl_lines="4 5 6"
 services:
   - name: database
     image: mysql
     environment:
       - MYSQL_DATABASE=test
       - MYSQL_ALLOW_EMPTY_PASSWORD=yes

   - name: cache
     image: redis
```

## Detachment

Services always start at the beginning of a workflow, asynchronously to any other step.

With the `detach` option, the execution order can be controlled in a more granular way.
This setting allows using normal steps as implicit "service" steps which honor the specified execution order.
These steps will also terminate at the end of the workflow.

```yaml hl_lines="10"
 steps:
   - name: build
     image: golang
     commands:
       - go build
       - go test

   - name: database
     image: redis
     detach: true

   - name: test
     image: golang
     commands:
       - go test
```

## Initialization

Service containers require time to initialize and begin accepting connections.
If the service is not yet available when the first step tries to connect and using `detach` is not an option, adding a short `sleep` command can help:

```yaml hl_lines="5"
 steps:
   - name: test
     image: golang
     commands:
       - sleep 15
       - go get
       - go test

 services:
   - name: database
     image: mysql
```

## Full example

```yaml
services:
  - name: database
    image: mysql
    environment:
      - MYSQL_DATABASE=test
      - MYSQL_ROOT_PASSWORD=example
steps:
  - name: get-version
    image: ubuntu
    commands:
      - ( apt update && apt dist-upgrade -y && apt install -y mysql-client 2>&1 )> /dev/null
      - sleep 20s # need to wait for mysql-server init
      - echo 'SHOW VARIABLES LIKE "version"' | mysql -u root -h database test -p example
```
