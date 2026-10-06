# File Operations Test

This example demonstrates the new file operation tags in docci.

## Create a new HTML file

<!-- docci file="example.html" reset-file -->

```html
<!DOCTYPE html>
<html>
    <head>
        <title>My Titlee</title>
    </head>
    <body>
        <h1>Welcome</h1>
    </body>
</html>
```

## Verify the file was created

<!-- docci output-contains="<!DOCTYPE html>" -->

```bash
cat example.html
```

## Fix the typo in the title (line 4)

<!-- docci file="example.html" line-replace="4" -->

```html
        <title>My Title</title>
```

## Verify the typo was fixed

<!-- docci output-contains="My Title" -->

```bash
grep "title" example.html
```

## Insert content after the h1 tag (line 7)

<!-- docci file="example.html" line-insert="7" -->

```html
        <p>This is a paragraph</p>
        <p>This is another paragraph</p>
```

## Verify the paragraphs were added

<!-- docci output-contains="This is a paragraph" -->

```bash
cat example.html
```

## Create a CSS file

<!-- docci file="styles.css" -->

```css
body {
    font-family: Arial, sans-serif;
    margin: 0;
    padding: 20px;
}

h1 {
    color: #333;
}
```

## Add more styles at the end

<!-- docci file="styles.css" line-insert="10" -->

```css

p {
    line-height: 1.6;
    color: #666;
}
```

## Replace the h1 color (line 8)

<!-- docci file="styles.css" line-replace="8" -->

```css
    color: #0066cc;
```

## Verify the final CSS

<!-- docci output-contains="color: #0066cc" -->

```bash
cat styles.css
```

## Test conditional file creation

<!-- docci if-file-not-exists="example.html" -->

```bash
echo "This should not run because example.html exists"
```

## Clean up

```bash
rm -f example.html styles.css
echo "Test files cleaned up"
```
