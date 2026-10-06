# Docci | Readme Test 🚀 &middot; [![GitHub license](https://img.shields.io/badge/license-apache-blue.svg)](https://github.com/Reecepbcups/docci/blob/main/LICENSE) [![Tests](https://github.com/Reecepbcups/docci/actions/workflows/go-unit-test.yml/badge.svg)](https://github.com/Reecepbcups/docci/actions/workflows/test.yml) [![Compatible](https://img.shields.io/badge/compatible%20-macOS_&_linux-8A2BE2.svg)](https://github.com/Reecepbcups/docci)

Your documentation is now your test suite! 🎯 *(pronounced "doc-ee", short for documentation CI)*

A CI tool that brings your markdown docs to life by executing code blocks in sequence. Run processes in the background, handle environment variables, add delays, verify outputs, and modify files - all through simple markdown tags. Perfect for ensuring your docs stay accurate and your examples actually work! 📚

## 🌟 Projects Using Docci

Several projects have adopted Docci to ensure their documentation stays accurate and executable:

- **[Spawn](https://github.com/rollchains/spawn)** - Rapid Cosmos blockchain development framework
- **[WAVS](https://github.com/Lay3rLabs/wavs-middleware)** - Eigenlayer AVS WebAssembly development
- **[OndoChain](https://ondo.finance/ondo-chain/)** - Institutional RWA Cosmos-EVM blockchain

## 🏃‍♂️ Quick Start

Find sample workspaces in the [`examples/` directory](./examples/).

### 📦 Installation

[Go `1.23`+](https://go.dev/doc/install) is required. You can also download a pre-built binary from the [release page](https://github.com/Reecepbcups/docci/releases).

<!-- docci ignore -->

```bash
go install github.com/reecepbcups/docci
```

<!-- docci ignore -->

```bash
git clone git@github.com:Reecepbcups/docci.git --depth 1
cd docci
go mod tidy
task install # go install ./*.go
```

### 🤖 Github Actions Integration
````yaml
  # docci_Linux_x86_64, docci_Linux_arm64, docci_Darwin_x86_64, docci_Darwin_arm64
  - name: Install Docci Readme Test Tool
    run: |
      VERSION=v0.9.2
      BINARY=docci_Linux_x86_64.tar.gz
      curl -fsSL "https://github.com/Reecepbcups/docci/releases/download/${VERSION}/${BINARY}" | sudo tar -xzC /usr/local/bin
      sudo chmod +x /usr/local/bin/docci

  - run: docci run YOUR_MARKDOWN_FILE.md --hide-background-logs
````

### 🎮 Usage

<!-- docci ignore -->

```bash
docci run <markdown_file.md> [options]

docci run nested/README.md --hide-background-logs
docci run A.md --cleanup-commands "docker-compose down" --cleanup-commands "rm -rf /tmp/test"
docci run A.md --pre-commands "npm install"

docci tags

docci version
```

### 📍 Where directives go

Directives live in a comment above the block they describe, never in the fence
info string. That keeps the info string for the renderer -- `title=`, `icon=`,
`highlight={1-3}` -- and keeps test scaffolding out of the published page.

Use the JSX form on MDX pages and the HTML form on plain markdown. An HTML
comment is a parse error in MDX and a JSX comment is literal text in markdown,
so write the form the page needs; docci reads both.

````markdown
<!-- docci retry=3 -->

```bash
curl -s localhost:3000/health
```
````

Several directives can share one comment, on one line or many:

````markdown
{/* docci
  retry=5
  delay-per-cmd=1
  output-contains="ok"
*/}

```bash
curl -s localhost:3000/health
```
````

A comment attaches to the code block directly below it. Only blank lines and
other docci comments may come between. A comment that attaches to nothing is an
error, not a silent skip -- a test tool that quietly stops testing is worse than
one that fails.

### 🎨 Operation directives
  * 🛑 `ignore`: Skip executing this code block
  * 🏷️ `name="..."`: Name the block, so failures say which step broke
  * 🔄 `background`: Run the command in the background
  * 💀 `background-kill=N`: Kill a previously started background process by index (1-based)
  * 🚫 `if-not-installed=BINARY`: Skip execution if some binary is installed (e.g. node)
  * ⏲️ `delay-before=N`: Wait N seconds before running any commands in the block
  * ⏲️ `delay-after=N`: Wait N seconds after running all commands in the block
  * ⌛ `delay-per-cmd=N`: Wait N seconds before each command
  * ⏲️ `retry=N`: Retry the block N times. Paired with `output-contains` or
    `expect-output` it retries until that check passes, which is what a step
    that polls needs -- the command exits cleanly while reporting a pending
    state, so retrying on exit code alone never waits for anything
  * 🌐 `wait-for-endpoint=http://localhost:8080/health|N`: Wait up to N seconds for the endpoint to be ready
  * 📜 `output-contains="string"`: Ensure the output contains a string at the end of the block. Output means stdout and stderr together, the way the reader sees it in their terminal
  * ✅ `expect-output`: Mark the block as the expected output of the block above it
  * 🚨 `assert-failure`: If it is expected to fail (non 0 exit code)
  * 🖥️ `os=mac|linux`: Run the command only on the specified OS
  * 🔄 `replace-text="old;new"`: Replace text in the code block before execution (including env variables!)

### 📄 File directives
  * 📝 `file`: The file name to operate on
  * 🔄 `reset-file`: Reset the file to its original content
  * 🚫 `if-file-not-exists`: Only run if a file does not exist
  * ➕ `line-insert=N`: Insert content at line N
  * ✏️ `line-replace=N`: Replace content at line N
  * 📋 `line-replace=N-M`: Replace content from line N to M

### ✅ Checking output two ways

`output-contains` is a **tripwire**: the page fails if the step stops working,
and the reader never sees the check. Use it when the output is noise to a
reader -- an exit code, a `docker ps` line, a file that now exists.

`expect-output` is a **checkpoint**: the block renders on the page for the
reader to compare their terminal against, *and* docci asserts against the same
text. One artifact, rendered and tested, so there is no pasted sample sitting
there rotting.

````markdown
<!-- docci name="chain is producing blocks" retry=5 delay-per-cmd=1 -->

```bash
curl -s localhost:26657/status | jq '.result.sync_info'
```

<!-- docci expect-output -->

```json
{
  "latest_block_height": "<...>",
  "catching_up": false
}
```
````

`<...>` stands for a span that changes between runs -- a height, a hash, a
timestamp. It reads as a placeholder to a human and matches anything for docci.
Write `\<...>` for output that really does contain `<...>`.

Matching is "contains", not equality, and whitespace is collapsed on both
sides: real output is surrounded by logs and prompts that no page should have
to reproduce.

Add `retry` when the output takes a while to arrive:

````markdown
<!-- docci name="transfer reaches a terminal state" retry=10 delay-per-cmd=2 -->

```bash
ibc relayer packets --chain-id 41001 --tx-hash "$TX"
```

<!-- docci expect-output -->

```json
{"state": "PACKET_STATE_SUCCEEDED", "sequenceNumber": "<...>"}
```
````

The block runs until its expected output appears or the attempts run out. Each
attempt is printed, so a CI log shows the state changing rather than one opaque
wait.

### 🔌 Checking a background process

A long-running process has no exit code to wait on and no block output to
validate afterwards, so its startup output used to be the one thing on a page
that nothing checked. Give a background block an `expect-output` and docci
waits for that text to appear in the process's log before moving on.

````markdown
<!-- docci name="start the relayer" background -->

```bash
ibc relayer run
```

It prints a readiness line once both chains are connected:

<!-- docci expect-output -->

```
level=INFO msg=Readiness readiness="{Event:ready ChainsConnected:[<...>]}"
```
````

This replaces a guessed `delay-after`, and it is better than
`wait-for-endpoint` when the process says in its own words that it is ready.
If the process exits first, or the line never appears within sixty seconds, the
page fails and its log is printed.

### 📑 Page-level config

A page can carry docci config in its YAML frontmatter. Mintlify and other MDX
docs frameworks ignore frontmatter keys they do not recognise, so this costs
nothing in the rendered page.

````yaml
---
title: "Deploy a contract"
docci:
  needs:
    - /docs/tutorials/run-a-local-chain
  cleanup:
    - pkill -f evmd || true
  working-dir: .
  runnable: true
---
````

  * `needs`: pages this one builds on. They run, in order, before this page, in
    the same shell. A path starting with `/` resolves from the repository root;
    anything else is relative to the page. Running the real prerequisite is
    slower than a setup script, but a setup script is a second source of truth
    for how you reach that state, and nothing tells you when it has drifted.
  * `cleanup`: commands to run after the page finishes, pass or fail. Cleanup is
    not instruction to the reader, so it does not belong in a step -- but CI
    must not leave state behind for the next page.
  * `working-dir`: directory to change into, relative to the page.
  * `runnable`: set `false` for a page whose blocks are illustrative. A page
    carrying directives is runnable by default.

### 💡 Code Block Tag Examples (Operations)

Skip needless installations if you are already set up: 🛑

<!-- The 4 backticks is just so it wraps in githubs UI, real test are written normally with the nested part (just 3 backticks) -->
````bash
<!-- docci os=linux if-not-installed=node -->

```bash
# this only runs if `node` is not found in the system & it's a linux system
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.2/install.sh | bash
export NVM_DIR="$HOME/.nvm"
nvm install v21.7.3
```
````

Ensure the output (stdout or stderr) contains a specific string: 📜

````bash
<!-- docci contains="xyzMyOutput" -->

```bash
echo xyzMyOutput
```
````

Run blocking commands in the background: 🌐

````bash
<!-- docci background delay-after=2 -->

```bash
go run examples/server_endpoint/test_server.go 3000
```
````

Add delays between commands for stability after the endpoint from a previous command is up: ⏱️

````bash
<!-- docci output-contains="GOOD" wait-for-endpoint=http://localhost:3000/health|5 -->

```bash
VALUE=$(curl http://localhost:3000/health)
echo "Got value: $VALUE"
```
````


Assert that a command fails: 🚨

````bash
<!-- docci assert-failure -->

```bash
notinstalledbin --version
```
````

Set ENV Variables

````bash
```bash
export SOME_ENV_VAR="abcdef"
OTHER_ENV_VAR="ghijkl"
echo "SOME_ENV_VAR is $SOME_ENV_VAR and OTHER_ENV_VAR is $OTHER_ENV_VAR"
```
````

Docci automatically sets `IS_DOCCI_RUN=true` for all executed commands:

````bash
```bash
# This environment variable is automatically set by docci
if [ "$IS_DOCCI_RUN" = "true" ]; then
    echo "Running inside docci!"
fi
```
````

Replace text before execution (useful for CI/CD): 🔄

````bash
<!-- docci replace-text="API_KEY;$SOME_ENV_VAR" -->

```bash
echo "Imagine a cURL request with API_KEY here"
```
````

Cleanup demo server if running in the background:

````bash
```bash
curl http://localhost:3000/kill
```
````

### 💡 Files Code Block Tag Examples

Create a new file from content: 📝

<!-- yes, the typo is meant to be here -->
````html
<!-- docci file=example.html reset-file -->

```html
<html>
    <head>
        <title>My Titlee</title>
    </head>
</html>
```
````

Replace the typo'ed line:

````html
<!-- docci file=example.html line-replace=3 -->

```html
        <title>My Title</title>
```
````

Add new content

````html
<!-- docci file=example.html line-insert=4 -->

```html
    <body>
        <h1>My Header</h1>
        <p>1 paragraph</p>
        <p>2 paragraph</p>
    </body>
```
````

Replace multiple lines

````html
<!-- docci file=example.html line-replace=7-9 -->

```html
        <p>First paragraph</p>
        <p>Second paragraph</p>
```
````
