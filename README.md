# Hest

A leaderboard and scorekeeping system for the legendary basketball game Knockout
(or Lightning), some even call it Hest (Danish for horse).

## Installation

Install it using [Go](https://go.dev/):

```bash
go install github.com/martinohansen/hest@latest
```

## Usage

Just run it, it will create a [SQLite](https://www.sqlite.org/) database in the workdir:

```bash
HEST_PASSWORD=<password> hest
```

## Configuration

| Variable | Default | Purpose |
|---|---|---|
| `HEST_HOST` | `localhost` | HTTP bind host. Use a hostname or an unbracketed IP address. |
| `HEST_PORT` | `8080` | HTTP listen port. |
| `HEST_PASSWORD` | `hest` | Basic authentication password for write operations. |
| `HEST_LATITUDE` | `55.6761` | Latitude used for weather lookups. |
| `HEST_LONGITUDE` | `12.5683` | Longitude used for weather lookups. |

Hest trusts the first `X-Forwarded-For` address. Sanitize it at the proxy.

## Screenshots

This is how it looks in action:

Leaderboard | Player | Add game | H2H |
:----------:|:------:|:--------:|:---:|
<img width="723" height="750" alt="Leaderboard" src="https://github.com/user-attachments/assets/b67b1c74-a67d-447a-8f21-9984ebd4e9f2" /> | <img width="715" height="816" alt="Player" src="https://github.com/user-attachments/assets/085b107e-fdf1-4383-a81a-8906a35191f4" /> | <img width="718" height="824" alt="Add game" src="https://github.com/user-attachments/assets/a1541a67-f880-46b2-9176-e553dc29cfd2" /> | <img width="718" height="809" alt="H2H" src="https://github.com/user-attachments/assets/6018e421-a023-4512-a2bc-5a4c78d9456f" />

## Contributing

Pull requests are welcome.
