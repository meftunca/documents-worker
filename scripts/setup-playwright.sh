#!/bin/bash

# Playwright PDF Generator Setup Script
# This script installs Node.js dependencies and Playwright browsers

set -e

echo "🎭 Setting up Playwright PDF Generator..."

# Check if Node.js is installed
if ! command -v node &> /dev/null; then
    echo "❌ Node.js is not installed. Please install Node.js first:"
    echo "   macOS: brew install node"
    echo "   Ubuntu: sudo apt-get install nodejs npm"
    echo "   Or visit: https://nodejs.org/"
    exit 1
fi

echo "✅ Node.js version: $(node --version)"
echo "✅ NPM version: $(npm --version)"

# Navigate to playwright directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PLAYWRIGHT_DIR="$SCRIPT_DIR/playwright"

if [ ! -d "$PLAYWRIGHT_DIR" ]; then
    echo "❌ Playwright directory not found: $PLAYWRIGHT_DIR"
    exit 1
fi

cd "$PLAYWRIGHT_DIR"

echo "📦 Installing Node.js dependencies..."
npm install

echo "🌐 Installing Playwright browsers..."
npx playwright install chromium

echo "🧪 Testing Playwright installation..."
if node -e "const { chromium } = require('playwright'); console.log('✅ Playwright works!');" &> /dev/null; then
    echo "✅ Playwright installation successful!"
else
    echo "❌ Playwright installation failed!"
    exit 1
fi

echo ""
echo "🎉 Setup complete! You can now use Playwright PDF generation."
echo ""
echo "Test with:"
echo "  go test ./pdfgen -v -run TestPlaywright"
echo ""
echo "Environment variables:"
echo "  NODEJS_PATH=/path/to/node (default: node)"
echo "  PLAYWRIGHT_ENABLED=true|false (default: true)"
