# Test docci-if-not-installed tag

<!-- docci if-not-installed=ls -->

```bash
# This should not run because ls is already installed
exit 1
```

<!-- docci if-not-installed=nonexistent-fake-command -->

```bash
echo "Installing nonexistent-fake-command..."
echo "This command would install the fake command"
echo "Installation complete!"
```
