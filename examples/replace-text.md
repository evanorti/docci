# Replace Text Tag Test

## Basic

<!-- docci output-contains="Value is: 42" replace-text="PLACEHOLDER;42" -->

```bash
echo "Value is: PLACEHOLDER"
```

## Multiple Occurrences

<!-- docci replace-text="XXX;YYY" -->

```bash
echo "XXX appears here"
echo "And XXX appears here too"
echo "Even XXX appears a third time"
```

```bash
# imagine this was set in the CI env with github action secrets.
MY_SECRET_ENV_VAR="secret123"
```

## Replacement with an Environment Variable

<!-- docci output-contains="secret123" replace-text="SECRET_HERE;$MY_SECRET_ENV_VAR" -->

```bash
echo "SECRET_HERE"
```

## Replacement with the ; in the command

Complex replacement that puts multiple commands in 1 command, keeping the original

<!-- docci output-contains="xyz" replace-text="abc;echo abc;echo xyz" -->

```bash
echo "abc"
```
