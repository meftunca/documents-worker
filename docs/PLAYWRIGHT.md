# 🎭 Playwright PDF Generator

Modern, high-quality PDF generation using Playwright and Chromium browser engine.

## ✨ Features

- **Modern Rendering**: Full CSS3, JavaScript, and modern web standards support
- **High Performance**: Much faster than legacy tools like wkhtmltopdf
- **Better Quality**: Perfect rendering of complex layouts, gradients, and modern CSS
- **Reliable**: Consistent results across different environments
- **Flexible**: Support for HTML content, files, and URLs

## 🚀 Quick Setup

1. **Install Dependencies**:
   ```bash
   ./scripts/setup-playwright.sh
   ```

2. **Verify Installation**:
   ```bash
   go test ./pdfgen -v -run TestPlaywright
   ```

## 📖 Usage Examples

### Basic HTML to PDF
```go
generator := NewPDFGenerator(config)

htmlContent := `
<!DOCTYPE html>
<html>
<body>
    <h1>Modern PDF</h1>
    <p>Generated with Playwright!</p>
</body>
</html>
`

options := &GenerationOptions{
    PageSize:    "A4",
    Orientation: "portrait",
    Margins: map[string]string{
        "top": "1cm", "bottom": "1cm",
        "left": "1cm", "right": "1cm",
    },
}

result, err := generator.GenerateFromHTMLWithPlaywright(htmlContent, options)
```

### URL to PDF
```go
result, err := generator.GenerateFromURLWithPlaywright(
    "https://example.com", 
    options,
)
```

### Advanced Options
```go
options := &GenerationOptions{
    PageSize:    "A4",
    Orientation: "landscape",
    Margins: map[string]string{
        "top": "2cm", "bottom": "2cm",
        "left": "1.5cm", "right": "1.5cm",
    },
}
```

## 🔧 Configuration

Environment variables:
- `NODEJS_PATH`: Path to Node.js executable (default: `node`)
- `PLAYWRIGHT_ENABLED`: Enable Playwright generation (default: `true`)

## 🆚 Comparison: wkhtmltopdf vs Playwright

| Feature | wkhtmltopdf | Playwright |
|---------|-------------|------------|
| CSS3 Support | Limited | Full ✅ |
| JavaScript | No | Yes ✅ |
| Performance | Slow | Fast ⚡ |
| Modern Layouts | Poor | Excellent 🎨 |
| Maintenance | Deprecated | Active 🔄 |
| Font Rendering | Basic | High Quality 💎 |

## 🧪 Testing

```bash
# Test Playwright PDF generation
go test ./pdfgen -v -run TestPlaywright

# Test with custom options
go test ./pdfgen -v -run TestPlaywrightURL
```

## 🛠️ Troubleshooting

**Node.js not found:**
```bash
# macOS
brew install node

# Ubuntu
sudo apt-get install nodejs npm
```

**Playwright browsers missing:**
```bash
cd scripts/playwright
npx playwright install chromium
```

**Permission issues:**
```bash
chmod +x scripts/setup-playwright.sh
```
