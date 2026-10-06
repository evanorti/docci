# Validation Test

```bash
echo "Hello World"
```

<!-- docci output-contains="test value" -->

```bash
VAR="test value"
echo "This contains $VAR"
```

<!-- docci output-contains="Success" -->

```bash
echo "Success: All tests passed!"
```
