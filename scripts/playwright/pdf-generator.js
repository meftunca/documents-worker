#!/usr/bin/env node

const { chromium } = require('playwright');
const fs = require('fs');
const path = require('path');

/**
 * Playwright PDF Generator
 * Usage: node pdf-generator.js <inputFile> <outputFile> [options]
 */

async function generatePDF() {
    const args = process.argv.slice(2);
    
    if (args.length < 2) {
        console.error('Usage: node pdf-generator.js <inputFile> <outputFile> [optionsJSON]');
        process.exit(1);
    }

    const inputFile = args[0];
    const outputFile = args[1];
    const optionsJSON = args[2] || '{}';
    
    let options = {};
    try {
        options = JSON.parse(optionsJSON);
    } catch (e) {
        console.error('Invalid options JSON:', e.message);
        process.exit(1);
    }

    // Default PDF options
    const pdfOptions = {
        path: outputFile,
        format: options.pageSize || 'A4',
        landscape: options.orientation === 'landscape',
        margin: {
            top: options.marginTop || '1cm',
            right: options.marginRight || '1cm',
            bottom: options.marginBottom || '1cm',
            left: options.marginLeft || '1cm'
        },
        printBackground: true,
        preferCSSPageSize: false,
        ...options.pdfOptions
    };

    let browser = null;
    
    try {
        // Launch browser
        browser = await chromium.launch({
            headless: true,
            args: ['--no-sandbox', '--disable-dev-shm-usage']
        });

        const page = await browser.newPage();
        
        // Set viewport for consistent rendering
        await page.setViewportSize({ 
            width: options.viewportWidth || 1200, 
            height: options.viewportHeight || 800 
        });

        // Load content
        if (inputFile.startsWith('http://') || inputFile.startsWith('https://')) {
            // URL input
            await page.goto(inputFile, { 
                waitUntil: 'networkidle',
                timeout: options.timeout || 30000
            });
        } else if (fs.existsSync(inputFile)) {
            // File input
            const content = fs.readFileSync(inputFile, 'utf8');
            await page.setContent(content, { 
                waitUntil: 'networkidle',
                timeout: options.timeout || 30000
            });
        } else {
            // Direct HTML content
            await page.setContent(inputFile, { 
                waitUntil: 'networkidle',
                timeout: options.timeout || 30000
            });
        }

        // Wait for any additional elements if specified
        if (options.waitForSelector) {
            await page.waitForSelector(options.waitForSelector, {
                timeout: options.timeout || 30000
            });
        }

        if (options.waitTime) {
            await page.waitForTimeout(options.waitTime);
        }

        // Add custom CSS if provided
        if (options.css) {
            await page.addStyleTag({ content: options.css });
        }

        // Generate PDF
        await page.pdf(pdfOptions);

        // Get file stats
        const stats = fs.statSync(outputFile);
        const result = {
            success: true,
            outputPath: outputFile,
            fileSize: stats.size,
            generatedAt: new Date().toISOString(),
            options: pdfOptions
        };

        console.log(JSON.stringify(result));

    } catch (error) {
        const errorResult = {
            success: false,
            error: error.message,
            stack: error.stack
        };
        console.error(JSON.stringify(errorResult));
        process.exit(1);
    } finally {
        if (browser) {
            await browser.close();
        }
    }
}

// Handle process signals
process.on('SIGINT', async () => {
    console.error('Process interrupted');
    process.exit(1);
});

process.on('SIGTERM', async () => {
    console.error('Process terminated');
    process.exit(1);
});

// Run the generator
generatePDF().catch((error) => {
    console.error(JSON.stringify({
        success: false,
        error: error.message,
        stack: error.stack
    }));
    process.exit(1);
});
