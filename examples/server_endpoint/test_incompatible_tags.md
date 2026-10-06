# Test Incompatible Tag Combinations

This test should fail because docci-wait-for-endpoint and docci-background cannot be used together:

<!-- docci wait-for-endpoint="http://localhost:8080/health|10" background -->

```bash
echo "This should not work"
```