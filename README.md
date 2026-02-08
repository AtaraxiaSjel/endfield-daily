# Endfield Daily Check-in

A lightweight, standalone Go utility designed to automate daily attendance check-ins for *Arknights: Endfield*. This tool supports multiple accounts and provides status reports via Telegram.

It is designed to be easily deployed on Linux servers or any environment capable of running a Go binary.

## Configuration

The application requires a `config.toml` file to run. By default, it looks for this file in the current working directory, or you can specify a path via the `CONFIG_PATH` environment variable.

### Example `config.toml`

```toml
[telegram]
enabled = true
bot_token = "123456789:AbCdEfGhIjKlMnOpQrStUvWxYz"
chat_id = "987654321"

# Profile 1
[[profiles]]
account_name = "MainAccount"
cred = "YOUR_CRED_TOKEN"
sk_game_role = "123456"
platform = "3"
v_name = "1.0.0"

# Profile 2 (Optional)
[[profiles]]
account_name = "AltAccount"
cred = "YOUR_SECOND_CRED_TOKEN"
sk_game_role = "654321"
platform = "3"
v_name = "1.0.0"
```

## How to Obtain Credentials

See [this gist](https://gist.github.com/cptmacp/1e9a9f20f69c113a0828fea8d13cb34c?permalink_comment_id=5959869#gistcomment-5959869) for guide how to obtain skport credentials.

## Building and Running

### Standard Go Build
If you have Go installed (1.21+), you can build the binary directly:

```bash
go mod tidy
go build -o endfield-daily
./endfield-daily
```

### Nix / NixOS
This project uses Nix Flakes. To build the package:

```bash
nix build
```

The binary will be available in `./result/bin/endfield-daily`.
