---
title: "completion"
weight: 12
---

Use `tkn pac completion` to generate shell completion scripts for Bash, Zsh, Fish, or PowerShell. Shell completions let you press Tab to auto-complete subcommands and flags.

## Usage

```shell
tkn pac completion [bash|zsh|fish|powershell]
```

## Loading Completions

### Bash

Load completions in the current session:

```shell
source <(tkn pac completion bash)
```

To load completions automatically for every session, run once:

```shell
# Linux
tkn pac completion bash > /etc/bash_completion.d/tkn-pac

# macOS
tkn pac completion bash > $(brew --prefix)/etc/bash_completion.d/tkn-pac
```

### Zsh

Load completions in the current session:

```shell
source <(tkn pac completion zsh)
```

To load completions automatically for every session, run once:

```shell
tkn pac completion zsh > "${fpath[1]}/_tkn-pac"
```

### Fish

Load completions in the current session:

```shell
tkn pac completion fish | source
```

To load completions automatically for every session, run once:

```shell
tkn pac completion fish > ~/.config/fish/completions/tkn-pac.fish
```
