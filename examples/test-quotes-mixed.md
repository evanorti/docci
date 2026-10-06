# Test Mixed Quote Cases

Test various quote combinations:

<!-- docci output-contains='"operators": []' -->

```bash
echo '{"operators": [], "test": "value"}'
```

<!-- docci output-contains="simple text" -->

```bash
echo "This is simple text without quotes"
```

<!-- docci output-contains='text with "quotes" inside' -->

```bash
echo 'Here is text with "quotes" inside it'
```
