# Gator
An RSS feed aggre*gator*. (a [Boot.dev](https://www.boot.dev/lessons/55d273f2-5d8f-4acc-9493-bf8be8aea700) project)

## Requirements
You will need [Postgres](https://github.com/lib/pq) and [Go](https://go.dev/doc/install).

## Install
You can clone this repo and run this command at its root in your local computer:

```
git clone https://codeberg.org/kedarui/gator.git
cd path/to/gator
go install .
```

Or, you can start with running this command directly in your terminal:

```
go install codeberg.org/kedarui/gator@latest
```

## Setup

### 1. Database
Ensure you have `postgres` installed and a PostgresSQL database already running. Then open the `psql` shell:
```
#Linux
sudo -u postgres psql

#macOS
psql postgres

```

Create a database for `gator`:
```sql
CREATE DATABASE gator;
```

Then create the tables by running the migrations from the root of this repo. You can use whichever migration tool you want. In my case, I used, [goose](https://github.com/pressly/goose):
```
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -dir sql/schema postgres <postgres_connection_string> up
```

Replace `<user>` and `<password>` with your local Postgres SQL credentials. And replace `<dbname>` with the database name you created  for this program (e.g. `gator`).


### 2. Database
Create a file named `.gatorconfig.json` in your home directory. It should contain the following:

```json
{
    "db_url": "postgres://<user>:<password>@localhost:5432/<dbname>?sslmode=disable"
}
```

Use the same credentials as the step above. Gator adds `current_user_name` to this file itself when you register or log in.

## Usage
Run the program with:

```
gator <command> [arguments]
```

### Commands
- `gator register <name>`: create a user and log in
- `gator login <name>`: switch to an existing user
- `gator users`: list all users
- `gator addfeed <name> <url>`: add a feed and follow it
- `gator feeds`: list all feeds
- `gator follow <url>`: follow a feed that has already been added 
- `gator unfollow <url>`: unfollow a feed
- `gator following`: list the feeds you follow
- `gator agg <duration>`: fetch posts from feeds, one feed every `<duration>` (e.g. `1m`, `10m`, `1h`)
- `gator browse [limit]`: show the latest posts (default: 2)
- `gator reset`: delete all users, along their feeds and posts (HANDLE WITH CARE)

### Example

```
gator register kedarui
gator addfeed "Boot.dev Blog" https://blog.boot.dev/index.xml
gator agg 10m
```

`agg` runs until you stop it with `Ctrl+C`. So you can leave it running in its own terminal, while using gator in another:

```
gator browse 5
```
