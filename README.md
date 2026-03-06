# Hest

A leaderboard and scorekeeping system for the legendary basketball game Knockout
(or Lightning), some even call it Hest (Danish for horse).

## Install

Install it using [Go](https://go.dev/):

```bash
go install github.com/martinohansen/hest@latest
```

## Usage

Just run it, it will create a [SQLite](https://www.sqlite.org/) database in the workdir:

```bash
HEST_PASSWORD=<password> hest
```
