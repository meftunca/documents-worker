#!/usr/bin/env python3
"""
PyMuPDF Bridge for Documents Worker
Converts PDF and Office documents to Markdown format
"""

import fitz  # PyMuPDF
import sys
import json
import argparse
import os
from pathlib import Path
import subprocess
import tempfile
import re
from datetime import datetime

class MarkdownConverter:
    def __init__(self):
        self.supported_formats = {
            '.pdf': self.pdf_to_markdown,
            '.docx': self.office_to_markdown,
            '.doc': self.office_to_markdown,
            '.pptx': self.office_to_markdown,
            '.ppt': self.office_to_markdown,
            '.xlsx': self.office_to_markdown,
            '.xls': self.office_to_markdown,
            '.odt': self.office_to_markdown,
            '.odp': self.office_to_markdown,
            '.ods': self.office_to_markdown,
        }

    def convert_to_markdown(self, input_path, output_path=None, options=None):
        """Main conversion method"""
        if not os.path.exists(input_path):
            raise FileNotFoundError(f"Input file not found: {input_path}")
        
        file_ext = Path(input_path).suffix.lower()
        if file_ext not in self.supported_formats:
            raise ValueError(f"Unsupported file format: {file_ext}")
        
        if output_path is None:
            output_path = str(Path(input_path).with_suffix('.md'))
        
        # Ensure output directory exists
        os.makedirs(os.path.dirname(output_path) if os.path.dirname(output_path) else '.', exist_ok=True)
        
        # Convert using appropriate method
        converter_func = self.supported_formats[file_ext]
        result = converter_func(input_path, output_path, options or {})
        
        return result

    def pdf_to_markdown(self, pdf_path, output_path, options):
        """Convert PDF to Markdown using PyMuPDF"""
        start_time = datetime.now()
        
        # Open PDF
        doc = fitz.open(pdf_path)
        markdown_content = []
        
        # Process each page
        for page_num in range(len(doc)):
            page = doc[page_num]
            
            # Extract text blocks with formatting
            blocks = page.get_text("dict")
            page_markdown = self._process_pdf_blocks(blocks, page_num + 1, options)
            
            if page_markdown.strip():
                if page_num > 0:
                    markdown_content.append(f"\n\n---\n*Page {page_num + 1}*\n\n")
                markdown_content.append(page_markdown)
        
        doc.close()
        
        # Combine all content
        full_markdown = "".join(markdown_content)
        
        # Clean up markdown
        full_markdown = self._clean_markdown(full_markdown)
        
        # Write to file
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(full_markdown)
        
        # Prepare result
        result = {
            'success': True,
            'input_path': pdf_path,
            'output_path': output_path,
            'conversion_type': 'pdf_to_markdown',
            'pages_processed': len(doc),
            'duration': (datetime.now() - start_time).total_seconds(),
            'file_size': os.path.getsize(output_path),
            'word_count': len(full_markdown.split()),
            'char_count': len(full_markdown),
            'metadata': {
                'generator': 'pymupdf',
                'source_pages': len(doc)
            }
        }
        
        return result

    def _process_pdf_blocks(self, blocks, page_num, options):
        """Process PDF text blocks and convert to markdown"""
        markdown_lines = []
        
        for block in blocks.get("blocks", []):
            if "lines" not in block:
                continue
                
            for line in block["lines"]:
                line_text = ""
                line_formatting = []
                
                for span in line.get("spans", []):
                    text = span.get("text", "").strip()
                    if not text:
                        continue
                    
                    # Analyze text formatting
                    font_size = span.get("size", 12)
                    font_flags = span.get("flags", 0)
                    
                    # Determine if it's a heading based on font size
                    if font_size > 16:
                        line_text = f"# {text}"
                    elif font_size > 14:
                        line_text = f"## {text}"
                    elif font_size > 12:
                        line_text = f"### {text}"
                    else:
                        # Apply formatting
                        if font_flags & 2**4:  # Bold
                            text = f"**{text}**"
                        if font_flags & 2**1:  # Italic
                            text = f"*{text}*"
                        
                        line_text += text + " "
                
                if line_text.strip():
                    markdown_lines.append(line_text.strip())
        
        return "\n\n".join(markdown_lines)

    def office_to_markdown(self, office_path, output_path, options):
        """Convert Office document to Markdown via pandoc"""
        start_time = datetime.now()
        
        try:
            # Use pandoc for conversion
            cmd = [
                'pandoc',
                '-f', self._get_pandoc_format(office_path),
                '-t', 'markdown',
                '--wrap=none',
                '--extract-media=./images',
                office_path,
                '-o', output_path
            ]
            
            # Add options
            if options.get('preserve_images', True):
                cmd.extend(['--extract-media', os.path.dirname(output_path) or '.'])
            
            if options.get('table_style'):
                cmd.extend(['--columns', str(options.get('columns', 80))])
            
            # Execute pandoc
            result = subprocess.run(cmd, capture_output=True, text=True, check=True)
            
            # Read generated markdown
            with open(output_path, 'r', encoding='utf-8') as f:
                markdown_content = f.read()
            
            # Clean up markdown
            markdown_content = self._clean_markdown(markdown_content)
            
            # Write cleaned content back
            with open(output_path, 'w', encoding='utf-8') as f:
                f.write(markdown_content)
            
            # Prepare result
            result_data = {
                'success': True,
                'input_path': office_path,
                'output_path': output_path,
                'conversion_type': 'office_to_markdown',
                'duration': (datetime.now() - start_time).total_seconds(),
                'file_size': os.path.getsize(output_path),
                'word_count': len(markdown_content.split()),
                'char_count': len(markdown_content),
                'metadata': {
                    'generator': 'pandoc',
                    'source_format': Path(office_path).suffix
                }
            }
            
            return result_data
            
        except subprocess.CalledProcessError as e:
            raise RuntimeError(f"Pandoc conversion failed: {e.stderr}")
        except Exception as e:
            raise RuntimeError(f"Office conversion failed: {str(e)}")

    def _get_pandoc_format(self, file_path):
        """Get appropriate pandoc format for file"""
        ext = Path(file_path).suffix.lower()
        format_map = {
            '.docx': 'docx',
            '.doc': 'doc',
            '.odt': 'odt',
            '.pptx': 'pptx',
            '.ppt': 'ppt',
            '.odp': 'odp',
            '.xlsx': 'xlsx',
            '.xls': 'xls',
            '.ods': 'ods'
        }
        return format_map.get(ext, 'docx')

    def _clean_markdown(self, markdown_content):
        """Clean up and format markdown content"""
        # Remove excessive newlines
        markdown_content = re.sub(r'\n{3,}', '\n\n', markdown_content)
        
        # Fix heading spacing
        markdown_content = re.sub(r'\n(#{1,6})', r'\n\n\1', markdown_content)
        markdown_content = re.sub(r'(#{1,6}.*)\n([^\n#])', r'\1\n\n\2', markdown_content)
        
        # Fix list formatting
        markdown_content = re.sub(r'\n(\s*[-*+])', r'\n\n\1', markdown_content)
        
        # Clean up table formatting
        lines = markdown_content.split('\n')
        cleaned_lines = []
        prev_was_table = False
        
        for line in lines:
            is_table = '|' in line and line.strip().startswith('|')
            
            if is_table and not prev_was_table:
                cleaned_lines.append('')  # Add space before table
            elif not is_table and prev_was_table:
                cleaned_lines.append('')  # Add space after table
            
            cleaned_lines.append(line)
            prev_was_table = is_table
        
        return '\n'.join(cleaned_lines).strip()

    def batch_convert(self, input_dir, output_dir, options=None):
        """Convert all supported files in a directory"""
        input_path = Path(input_dir)
        output_path = Path(output_dir)
        
        output_path.mkdir(parents=True, exist_ok=True)
        
        results = []
        
        for file_path in input_path.rglob('*'):
            if file_path.is_file() and file_path.suffix.lower() in self.supported_formats:
                relative_path = file_path.relative_to(input_path)
                output_file = output_path / relative_path.with_suffix('.md')
                
                try:
                    result = self.convert_to_markdown(str(file_path), str(output_file), options)
                    results.append(result)
                except Exception as e:
                    results.append({
                        'success': False,
                        'input_path': str(file_path),
                        'error': str(e)
                    })
        
        return results

def main():
    parser = argparse.ArgumentParser(description='Convert documents to Markdown using PyMuPDF')
    parser.add_argument('input', help='Input file or directory path')
    parser.add_argument('-o', '--output', help='Output file or directory path')
    parser.add_argument('--batch', action='store_true', help='Batch process directory')
    parser.add_argument('--preserve-images', action='store_true', default=True, 
                       help='Preserve images during conversion')
    parser.add_argument('--json', action='store_true', help='Output result as JSON')
    
    args = parser.parse_args()
    
    converter = MarkdownConverter()
    
    try:
        if args.batch:
            output_dir = args.output or f"{args.input}_markdown"
            results = converter.batch_convert(args.input, output_dir, {
                'preserve_images': args.preserve_images
            })
        else:
            output_file = args.output or str(Path(args.input).with_suffix('.md'))
            results = converter.convert_to_markdown(args.input, output_file, {
                'preserve_images': args.preserve_images
            })
        
        if args.json:
            print(json.dumps(results, indent=2, default=str))
        else:
            if isinstance(results, list):
                print(f"Processed {len(results)} files")
                for result in results:
                    if result.get('success'):
                        print(f"✓ {result['input_path']} -> {result['output_path']}")
                    else:
                        print(f"✗ {result['input_path']}: {result.get('error')}")
            else:
                print(f"✓ Converted: {results['input_path']} -> {results['output_path']}")
                print(f"  Pages: {results.get('pages_processed', 'N/A')}")
                print(f"  Words: {results['word_count']}")
                print(f"  Duration: {results['duration']:.2f}s")
    
    except Exception as e:
        error_result = {'success': False, 'error': str(e)}
        if args.json:
            print(json.dumps(error_result))
        else:
            print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == '__main__':
    main()
