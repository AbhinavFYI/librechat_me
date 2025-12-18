"""
Usage: 
python chunking.py <markdown_file_path> <output_json_path> --name <name>
"""
import os
import json
import uuid
import re
import logging
import asyncio
from typing import List, Dict, Any, Optional
from sentence_transformers import SentenceTransformer
from keybert import KeyBERT
import requests
import urllib3
import aiofiles

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

def disable_ssl_verification():
    original_init = requests.Session.__init__

    def patched_init(self, *args, **kwargs):
        original_init(self, *args, **kwargs)
        self.verify = False

    requests.Session.__init__ = patched_init
# Set up logging
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s: %(message)s")
logger = logging.getLogger(__name__)
disable_ssl_verification()
# Regex patterns for cleaning
RE_IMAGE_MD = re.compile(r"!\[.*?\]\(.*?\)")
RE_LINK_MD = re.compile(r"\[(.*?)\]\((.*?)\)")
RE_HTML_TAG = re.compile(r"<[^>]+>")
RE_MARKDOWN_CHARS = re.compile(r"[*_`~|#>-]{1,}")


class MarkdownChunker:
    """Chunks markdown files using Docling's HybridChunker"""
    
    def __init__(self, model_name: Optional[str] = None):
        """Initialize the markdown chunker with embedding model"""
        if model_name is None:
            model_name = os.getenv("HF_DOCLING_MODEL", "intfloat/multilingual-e5-base")
        
        self.embedding_model = SentenceTransformer(model_name)
        self.keyword_extractor = KeyBERT(model=self.embedding_model)
        self.chunk_size = 1024
        self.overlap = 50
        self.markdown_content = None  # Store markdown content for table title extraction
    
    def clean_text_from_html_and_md(self, raw_text: str) -> str:
        """Remove HTML tags and Markdown artifacts, normalize whitespace."""
        if not raw_text:
            return ""
        s = raw_text
        s = RE_IMAGE_MD.sub("", s)
        s = RE_LINK_MD.sub(r"\1", s)
        s = RE_HTML_TAG.sub(" ", s)
        s = RE_MARKDOWN_CHARS.sub(" ", s)
        s = re.sub(r"\s+", " ", s).strip()
        return s
    
    async def extract_keywords(self, text: str, top_n: int = 15) -> List[Dict[str, str]]:
        """Extract keywords from text using KeyBERT (async wrapper)."""
        clean = self.clean_text_from_html_and_md(text)
        if not clean:
            return []
        
        try:
            # Run keyword extraction in thread pool to avoid blocking
            def _extract():
                keywords = self.keyword_extractor.extract_keywords(
                    clean,
                    keyphrase_ngram_range=(1, 3),
                    stop_words='english',
                    top_n=top_n
                )
                return [{"value": kw[0], "score": float(kw[1])} for kw in keywords]
            
            return await asyncio.to_thread(_extract)
        except Exception as e:
            logger.warning(f"Failed to extract keywords: {e}")
            return []
    
    async def process_markdown_file(
        self,
        markdown_file_path: str,
        output_json_path: str,
        id: Optional[str] = None,
        name: Optional[str] = None,
        # document_type: Optional[str] = None,
        # document_format: Optional[str] = None,
        # document_year: Optional[int] = None,
        # document_quarter: Optional[str] = None,
        metadata: Optional[Dict[str, Any]] = None
    ) -> int:
        """
        Process a markdown file and save chunks to JSON file.
        
        Args:
            markdown_file_path: Path to the markdown file
            output_json_path: Path to save the JSON output
            id: Optional document ID (generated if not provided)
            name: Optional document name (uses filename if not provided)
            document_type: Optional document type (e.g., 'Financial Report', 'Annual Report')
            document_format: Optional document format (e.g., 'pdf', 'xlsx', 'docx', 'md')
            document_year: Optional document year (e.g., 2023, 2024)
            document_quarter: Optional document quarter (e.g., Q1, Q2, Q3, Q4)
            metadata: Optional additional metadata to include in all chunks
            
        Returns:
            Number of chunks created
        """
        try:
            from docling.document_converter import DocumentConverter
            from docling.chunking import HybridChunker
            
            logger.info(f"Processing markdown file: {markdown_file_path}")
            
            # Handle output path - if it's a directory, create a filename
            if os.path.isdir(output_json_path):
                # Generate output filename based on input filename
                input_basename = os.path.basename(markdown_file_path)
                input_name_without_ext = os.path.splitext(input_basename)[0]
                output_filename = f"{input_name_without_ext}_chunks.json"
                output_json_path = os.path.join(output_json_path, output_filename)
                logger.info(f"Output is a directory, saving to: {output_json_path}")
            else:
                # Ensure the output directory exists
                output_dir = os.path.dirname(output_json_path)
                if output_dir and not os.path.exists(output_dir):
                    os.makedirs(output_dir, exist_ok=True)
                    logger.info(f"Created output directory: {output_dir}")
            
            # Generate document ID if not provided
            if id is None:
                id = str(uuid.uuid4())
            
            # # Get document name from filename if not provided
            if name is None:
                name = os.path.basename(markdown_file_path)
            
            # Initialize metadata dict if not provided
            if metadata is None:
                metadata = {}
            
            # Read markdown content for table title extraction
            async with aiofiles.open(markdown_file_path, 'r', encoding='utf-8') as f:
                self.markdown_content = await f.read()
            
            # Convert markdown file to Docling document
            document_converter = DocumentConverter()
            result = document_converter.convert(source=markdown_file_path)
            doc = result.document
            
            # Store document reference for table extraction
            self._current_doc = doc
            
            # Initialize HybridChunker
            chunker = HybridChunker(chunk_size=self.chunk_size, overlap=self.overlap)
            
            # Open JSON file for streaming writes
            output_file = await aiofiles.open(output_json_path, 'w', encoding='utf-8')
            
            # Write document header
            await output_file.write('{\n')
            await output_file.write(f'  "name": {json.dumps(name)},\n')
            await output_file.write(f'  "id": {json.dumps(id)},\n')
            # await output_file.write(f'  "document_type": {json.dumps(document_type if document_type else "markdown")},\n')
            # await output_file.write(f'  "document_format": {json.dumps(document_format if document_format else "md")},\n')
            # await output_file.write(f'  "document_year": {json.dumps(document_year)},\n')
            # await output_file.write(f'  "document_quarter": {json.dumps(document_quarter)},\n')
            await output_file.write('  "chunks": [\n')
            
            # Track chunks for writing
            Index = 0
            is_first_chunk = True
            
            # Process tables
            if hasattr(doc, 'tables') and doc.tables:
                logger.info(f"Found {len(doc.tables)} tables in document")
                for table_idx, table in enumerate(doc.tables):
                    try:
                        # Get page number for table
                        page_number = 1
                        if hasattr(table, 'prov') and table.prov:
                            for prov_item in table.prov:
                                if hasattr(prov_item, 'page_no'):
                                    page_number = prov_item.page_no
                                    break
                                elif hasattr(prov_item, 'page'):
                                    page_number = prov_item.page
                                    break
                        
                        # Extract table structure
                        table_data_raw = self._extract_table_structure(table)
                        if not table_data_raw:
                            logger.warning(f"No table data extracted for table {table_idx}")
                            continue
                        
                        
                        
                        # Extract table title from markdown and detect header rows
                        markdown_title, num_header_rows,subtitle= self._extract_markdown_table_title_and_header(table_idx, table_data_raw)
                        
                        # Get table title (fallback to old method if markdown extraction fails)
                        if markdown_title:
                            section_title = markdown_title
                        else:
                            section_title = self._extract_table_title(table, doc, page_number, table_idx, table_data_raw)
                        
                        # Extract header rows and data rows separately
                        # COMMENTED OUT: Header extraction - now headers are included in body
                        # header_rows = table_data_raw[:num_header_rows] if num_header_rows > 0 else []
                        # data_rows = table_data_raw[num_header_rows:]  # Body without headers
                        
                        # Generate unique table_id for this table
                        table_id = str(uuid.uuid4())
                                
                        
                        # Create TABLE chunk
                        table_metadata = {
                            "id": table_id,
                            "title": markdown_title if markdown_title else section_title,
                            # "header": header_rows,  # Header rows only 
                            "body": table_data_raw  # All rows including headers
                        }
                        
                        whole_table_chunk = {
                            "id": str(uuid.uuid4()),
                            "Index": Index,
                            "table": table_metadata,
                            "metadata": {
                                **metadata,
                                "section_title": section_title,
                                "content_type": "table",
                            }
                        }
                        
                        # Write chunk to file immediately
                        if not is_first_chunk:
                            await output_file.write(',\n')
                        else:
                            is_first_chunk = False
                        
                        chunk_json = json.dumps(whole_table_chunk, indent=2, ensure_ascii=False)
                        # Indent the chunk JSON
                        indented_chunk = '\n'.join('    ' + line for line in chunk_json.split('\n'))
                        await output_file.write(indented_chunk)
                        await output_file.flush()
                        
                        Index += 1
                        # logger.info(f"Extracted WHOLE table {table_idx + 1} '{section_title}' from page {page_number} with {len(data_rows)} data rows and {num_header_rows} header rows") 
                        logger.info(f"Extracted WHOLE table {table_idx + 1} '{section_title}' from page {page_number} with {len(table_data_raw)} total rows (headers included in body)")
                        
                    except Exception as e:
                        logger.warning(f"Failed to extract table {table_idx}: {e}")
            
            # Process text chunks using HybridChunker
            chunk_iter = chunker.chunk(dl_doc=doc)
            
            for i, chunk in enumerate(chunk_iter):
                chunk_text = chunk.text
                
                # Get enriched/contextualized text if available
                try:
                    enriched_text = chunker.contextualize(chunk=chunk)
                    if enriched_text and enriched_text != chunk_text:
                        chunk_text_for_embedding = self.clean_text_from_html_and_md(enriched_text)
                    else:
                        chunk_text_for_embedding = self.clean_text_from_html_and_md(chunk_text)
                except Exception as e:
                    logger.warning(f"Failed to contextualize chunk {i}: {e}")
                    chunk_text_for_embedding = self.clean_text_from_html_and_md(chunk_text)
                
                # Skip empty chunks
                if not chunk_text_for_embedding or len(chunk_text_for_embedding.strip()) < 10:
                    continue
                
                # Skip chunks that are primarily table content
                is_table_chunk = False
                if hasattr(chunk, 'meta') and chunk.meta and hasattr(chunk.meta, 'doc_items'):
                    for doc_item in chunk.meta.doc_items:
                        if hasattr(doc_item, 'label') and 'table' in str(doc_item.label).lower():
                            is_table_chunk = True
                            break
                
                if is_table_chunk:
                    logger.debug(f"Skipping chunk {i} as it's part of a table")
                    continue
                
                # Extract page number
                page_number = 1  # Default
                page_found = False
                
                # Try to extract from meta.doc_items
                if hasattr(chunk, 'meta') and chunk.meta and hasattr(chunk.meta, 'doc_items'):
                    doc_items = chunk.meta.doc_items
                    if doc_items and len(doc_items) > 0:
                        first_item = doc_items[0]
                        
                        if hasattr(first_item, 'prov') and first_item.prov:
                            for prov_item in first_item.prov:
                                if hasattr(prov_item, 'page_no'):
                                    page_number = prov_item.page_no
                                    page_found = True
                                    break
                                elif hasattr(prov_item, 'page'):
                                    page_number = prov_item.page
                                    page_found = True
                                    break
                        
                        if not page_found and hasattr(first_item, 'bbox'):
                            bbox = first_item.bbox
                            if hasattr(bbox, 'page'):
                                page_number = bbox.page
                                page_found = True
                
                # Check if meta has direct page attribute
                if not page_found and hasattr(chunk.meta, 'page'):
                    page_number = chunk.meta.page
                    page_found = True
                
                if not page_found and hasattr(chunk.meta, 'page_no'):
                    page_number = chunk.meta.page_no
                    page_found = True
                
                # Extract section title
                section_title = "Unknown Section"
                
                if hasattr(chunk, 'meta') and chunk.meta:
                    if 'section' in chunk.meta and chunk.meta['section']:
                        section_title = self.clean_text_from_html_and_md(str(chunk.meta['section']))[:100]
                    elif hasattr(chunk.meta, 'headings') and chunk.meta.headings:
                        heading = chunk.meta.headings[-1] if isinstance(chunk.meta.headings, list) else str(chunk.meta.headings)
                        section_title = self.clean_text_from_html_and_md(str(heading))[:100]
                    elif hasattr(chunk.meta, 'heading') and chunk.meta.heading:
                        section_title = self.clean_text_from_html_and_md(str(chunk.meta.heading))[:100]
                
                # Try to extract from doc_items if still "Unknown"
                if section_title == "Unknown Section" and hasattr(chunk, 'meta') and chunk.meta and hasattr(chunk.meta, 'doc_items'):
                    for item in chunk.meta.doc_items:
                        item_label = str(getattr(item, 'label', '')).lower()
                        if any(h in item_label for h in ['heading', 'title', 'section']):
                            if hasattr(item, 'text') and item.text:
                                section_title = self.clean_text_from_html_and_md(str(item.text))[:100]
                            break
                
                # Try to extract from first line as last resort
                if section_title == "Unknown Section":
                    first_lines = chunk_text.split('\n')[:3]
                    for line in first_lines:
                        line = line.strip()
                        if line and (line.isupper() or line.startswith('#')):
                            section_title = self.clean_text_from_html_and_md(line)[:100]
                            break
                        elif len(line) > 10 and len(line) < 100:
                            section_title = self.clean_text_from_html_and_md(line)[:100]
                            break
                
                # Create chunk in specified format
                chunk_obj = {
                    "id": str(uuid.uuid4()),
                    "content": chunk_text_for_embedding,
                    "Index": Index,
                    "metadata": {
                        **metadata,
                        "section_title": section_title[:100],
                        "content_type": "text",
                    }
                }
                
                # Write chunk to file immediately
                if not is_first_chunk:
                    await output_file.write(',\n')
                else:
                    is_first_chunk = False
                
                chunk_json = json.dumps(chunk_obj, indent=2, ensure_ascii=False)
                # Indent the chunk JSON
                indented_chunk = '\n'.join('    ' + line for line in chunk_json.split('\n'))
                await output_file.write(indented_chunk)
                await output_file.flush()
                
                Index += 1
            
            # Close JSON structure
            await output_file.write('\n  ]\n')
            await output_file.write('}\n')
            await output_file.close()
            
            logger.info(f"Successfully created {Index} chunks and saved to {output_json_path}")
            return Index
            
        except Exception as e:
            logger.error(f"Error processing markdown file: {e}")
            import traceback
            logger.error(f"Traceback: {traceback.format_exc()}")
            
            # Close file if it was opened
            try:
                if 'output_file' in locals():
                    await output_file.close()
            except:
                pass
            
            return 0
    
    def _extract_table_structure(self, table) -> List[List[str]]:
        """Extract table structure from Docling table object"""
        try:
            table_data = []
            
            # Method 1: Try export_to_markdown
            if hasattr(table, 'export_to_markdown'):
                try:
                    doc = getattr(self, '_current_doc', None)
                    if doc:
                        markdown = table.export_to_markdown(doc=doc)
                    else:
                        markdown = table.export_to_markdown()
                    
                    if markdown:
                        table_data = self._parse_markdown_table(markdown)
                        if table_data:
                            return table_data
                except Exception as e:
                    logger.debug(f"Could not export table to markdown: {e}")
            
            # Method 2: Try export_to_dataframe
            if hasattr(table, 'export_to_dataframe'):
                try:
                    import pandas as pd
                    doc = getattr(self, '_current_doc', None)
                    if doc:
                        df = table.export_to_dataframe(doc=doc)
                    else:
                        df = table.export_to_dataframe()
                    
                    if df is not None and not df.empty:
                        headers = df.columns.tolist()
                        table_data.append([str(h) for h in headers])
                        
                        for _, row in df.iterrows():
                            row_data = [str(cell) if pd.notna(cell) else "" for cell in row]
                            table_data.append(row_data)
                        
                        if table_data:
                            return table_data
                except ImportError:
                    logger.debug("Pandas not available for export_to_dataframe")
                except Exception as e:
                    logger.debug(f"Could not export table to dataframe: {e}")
            
            # Method 3: Try to get from data attribute
            if hasattr(table, 'data') and table.data:
                if isinstance(table.data, dict) and 'grid' in table.data:
                    grid = table.data['grid']
                    for row in grid:
                        row_data = []
                        for cell in row:
                            cell_text = str(cell).strip() if cell else ""
                            row_data.append(cell_text)
                        table_data.append(row_data)
                elif isinstance(table.data, list):
                    for row in table.data:
                        if isinstance(row, list):
                            row_data = [str(cell).strip() if cell else "" for cell in row]
                            table_data.append(row_data)
                
                if table_data:
                    return table_data
            
            # Method 4: Try text representation
            if hasattr(table, 'text') and table.text:
                text = table.text
                lines = text.split('\n')
                for line in lines:
                    if line.strip():
                        cells = [cell.strip() for cell in line.split('|') if cell.strip()]
                        if not cells:
                            cells = [cell.strip() for cell in line.split('\t') if cell.strip()]
                        if cells and len(cells) > 1:
                            table_data.append(cells)
                
                if table_data:
                    return table_data
            
            # Method 5: Extract from grid property
            if hasattr(table, 'grid') and table.grid:
                for row in table.grid:
                    row_data = []
                    for cell in row:
                        if hasattr(cell, 'text'):
                            cell_text = str(cell.text).strip()
                        else:
                            cell_text = str(cell).strip() if cell else ""
                        row_data.append(cell_text)
                    if row_data:
                        table_data.append(row_data)
                
                if table_data:
                    return table_data
            
            logger.warning(f"Could not extract table structure using any method")
            return []
            
        except Exception as e:
            logger.error(f"Error extracting table structure: {e}")
            return []
    
    def _parse_markdown_table(self, markdown: str) -> List[List[str]]:
        """Parse a markdown table into a list of rows"""
        lines = markdown.strip().split('\n')
        table_data = []
        
        for line in lines:
            line = line.strip()
            if not line:
                continue
            
            # Skip separator lines
            if all(c in '-|: ' for c in line):
                continue
            
            # Split by pipes and clean up
            cells = [cell.strip() for cell in line.split('|')]
            if cells and not cells[0]:
                cells = cells[1:]
            if cells and not cells[-1]:
                cells = cells[:-1]
            
            if cells:
                table_data.append(cells)
        
        return table_data
    
    def _table_to_markdown(self, table_data: List[List[str]]) -> str:
        """Convert table data to markdown format"""
        if not table_data:
            return ""
        
        # Find maximum width for each column
        col_widths = []
        max_cols = max(len(row) for row in table_data) if table_data else 0
        
        for col_idx in range(max_cols):
            max_width = 0
            for row in table_data:
                if col_idx < len(row):
                    max_width = max(max_width, len(str(row[col_idx])))
            col_widths.append(max(max_width, 3))
        
        # Build markdown table
        markdown_lines = []
        
        for row_idx, row in enumerate(table_data):
            padded_row = row + [''] * (max_cols - len(row))
            
            formatted_cells = []
            for col_idx, cell in enumerate(padded_row):
                cell_str = str(cell).ljust(col_widths[col_idx])
                formatted_cells.append(cell_str)
            
            markdown_lines.append('| ' + ' | '.join(formatted_cells) + ' |')
            
            # Add separator after first row
            if row_idx == 0:
                separator = '|' + '|'.join(['-' * (w + 2) for w in col_widths]) + '|'
                markdown_lines.append(separator)
        
        return '\n'.join(markdown_lines)
    
    def _remove_consecutive_duplicates(self, table_data: List[List[str]]) -> List[List[str]]:
        """
        Remove consecutive duplicate values in table rows.
        If a value appears multiple times consecutively in a row, keep only one.
        """
        if not table_data:
            return table_data
        
        cleaned_data = []
        for row in table_data:
            cleaned_row = []
            prev_value = None
            
            for cell in row:
                cell_str = str(cell).strip()
                # Only add if it's different from the previous value
                if cell_str != prev_value or not cell_str:
                    cleaned_row.append(cell_str)
                    prev_value = cell_str
                # If it's a duplicate and not empty, skip it
            
            # If all values were duplicates and we ended up with just one cell, keep the original
            if len(cleaned_row) >= 1:
                cleaned_data.append(cleaned_row)
        
        return cleaned_data
    
    def _table_to_text(self, table_data: List[List[str]]) -> str:
        """Convert table data to plain text format"""
        if not table_data:
            return ""
        
        text_lines = []
        for row in table_data:
            row_text = ' | '.join(str(cell) for cell in row)
            text_lines.append(row_text)
        
        return '\n'.join(text_lines)
    
    def _extract_markdown_table_title_and_header(self, table_idx: int, table_data: List[List[str]]) -> tuple:
        """
        Extract table title from markdown heading and identify header rows.
        
        Returns:
            tuple: (table_title, header_rows_count, subtitle)
        """
        try:
            if not self.markdown_content or not table_data:
                return None, 1, None
            
            # Split markdown into lines
            lines = self.markdown_content.split('\n')
            
            # Find all table starts in markdown
            table_starts = []
            in_table = False
            
            for i, line in enumerate(lines):
                line_stripped = line.strip()
                is_table_line = line_stripped.startswith('|') and not line_stripped.startswith('|--')
                
                if is_table_line and not in_table:
                    # Start of a new table
                    table_starts.append(i)
                    in_table = True
                elif not is_table_line and not line_stripped.startswith('|--'):
                    # Not a table line, reset in_table
                    in_table = False
            
            logger.debug(f"Found {len(table_starts)} tables in markdown, looking for table {table_idx}")
            
            if table_idx >= len(table_starts):
                logger.debug(f"Table index {table_idx} out of range, only {len(table_starts)} tables found")
                return None, 1, None
            
            table_start_idx = table_starts[table_idx]
            logger.debug(f"Table {table_idx} starts at line {table_start_idx}")
            
            # Look backwards for heading (##) and subtitle
            table_title = None
            subtitle = None
            potential_title_without_hash = None
            
            for i in range(table_start_idx - 1, max(-1, table_start_idx - 5), -1):
                line = lines[i].strip()
                
                # Skip empty lines
                if not line:
                    continue
                
                # Check for markdown heading (## prefix)
                if line.startswith('##'):
                    table_title = line.lstrip('#').strip()
                    logger.debug(f"Found table title with ##: {table_title}")
                    
                    # Check for subtitle (line between heading and table)
                    for j in range(i + 1, table_start_idx):
                        potential_subtitle = lines[j].strip()
                        if potential_subtitle and potential_subtitle.startswith('(') and potential_subtitle.endswith(')'):
                            subtitle = potential_subtitle
                            logger.debug(f"Found subtitle: {subtitle}")
                            break
                    break
                
                # Check for text that could be a title (not a table row, reasonable length)
                elif not line.startswith('|') and 10 < len(line) < 150 and not line.startswith('*'):
                    # Could be a title without ## marker
                    if not potential_title_without_hash:
                        potential_title_without_hash = line
                        logger.debug(f"Found potential title without ##: {potential_title_without_hash}")
            
            # If no ## heading found, use potential title without hash
            if not table_title and potential_title_without_hash:
                table_title = potential_title_without_hash
                logger.debug(f"Using title without ##: {table_title}")
            
            # Detect number of header rows by analyzing table structure
            num_header_rows = 1
            if len(table_data) >= 3:
                # Check if first 2-3 rows contain headers (dates, column names)
                header_indicators = ['2023', '2024', '2025', '2026', 'fy', 'year', 'march', 'quarter', 'q1', 'q2', 'q3', 'q4', 'june', 'september', 'december']
                
                # Check second row
                if len(table_data) >= 2:
                    second_row_text = ' '.join([str(cell).lower() for cell in table_data[1]])
                    if any(indicator in second_row_text for indicator in header_indicators):
                        num_header_rows = 2
                        logger.debug(f"Detected 2 header rows")
                
                # Check third row
                if len(table_data) >= 3 and num_header_rows == 2:
                    third_row_text = ' '.join([str(cell).lower() for cell in table_data[2]])
                    if any(indicator in third_row_text for indicator in header_indicators):
                        num_header_rows = 3
                        logger.debug(f"Detected 3 header rows")
            
            logger.debug(f"Returning: title='{table_title}', num_headers={num_header_rows}, subtitle='{subtitle}'")
            return table_title, num_header_rows, subtitle
            
        except Exception as e:
            logger.warning(f"Error extracting markdown table title: {e}")
            import traceback
            traceback.print_exc()
            return None, 1, None
    
    def _extract_table_title(self, table, doc, page_number: int, table_idx: int, table_data: List[List[str]] = None) -> str:
        """Extract the title/heading for a table"""
        try:
            # Method 1: Check for table caption
            if hasattr(table, 'caption') and table.caption:
                caption = str(table.caption).strip()
                if caption and len(caption) > 3:
                    title = self.clean_text_from_html_and_md(caption)
                    if title and not title.lower().startswith('table'):
                        return f"Table: {title}"[:100]
                    return title[:100]
            
            # Method 2: Check for title in metadata
            if hasattr(table, 'data') and isinstance(table.data, dict):
                if 'title' in table.data and table.data['title']:
                    title = self.clean_text_from_html_and_md(str(table.data['title']))
                    if title and len(title) > 3:
                        return title[:100]
            
            # Method 3: Look for heading near table
            if hasattr(doc, 'pages') and doc.pages and page_number > 0:
                try:
                    page_idx = page_number - 1
                    if page_idx < len(doc.pages):
                        page = doc.pages[page_idx]
                        
                        if hasattr(page, 'elements'):
                            heading_candidates = []
                            
                            for element in page.elements:
                                element_label = str(getattr(element, 'label', '')).lower()
                                is_heading = any(heading_type in element_label for heading_type in 
                                               ['heading', 'title', 'section', 'caption'])
                                
                                if hasattr(element, 'text') and element.text:
                                    element_text = str(element.text).strip()
                                    
                                    if len(element_text) < 3:
                                        continue
                                    
                                    if is_heading:
                                        heading_candidates.append(element_text)
                                    elif 'table' in element_text.lower() and len(element_text) < 150:
                                        heading_candidates.append(element_text)
                            
                            if heading_candidates:
                                title = self.clean_text_from_html_and_md(heading_candidates[-1])
                                if title and len(title) > 3:
                                    return title[:100]
                except Exception as e:
                    logger.debug(f"Error extracting heading from page elements: {e}")
            
            # Method 4: Check first row
            if table_data is None:
                table_data = self._extract_table_structure(table)
            
            if table_data and len(table_data) > 0:
                first_row = table_data[0]
                if len(first_row) == 1 and len(str(first_row[0])) > 10:
                    potential_title = str(first_row[0]).strip()
                    if not potential_title.replace('.', '').replace(',', '').replace(' ', '').isdigit():
                        title = self.clean_text_from_html_and_md(potential_title)
                        if title:
                            return title[:100]
            
            # Fallback
            return f"Page {page_number} - Table {table_idx + 1}"
            
            
        except Exception as e:
            logger.debug(f"Error extracting table title: {e}")
            return f"Page {page_number if page_number else 'Unknown'} - Table {table_idx + 1}"


async def main():
    """CLI interface for chunking markdown files (async)"""
    import argparse
    
    parser = argparse.ArgumentParser(description='Chunk markdown files using Docling HybridChunker')
    parser.add_argument('input_md', help='Path to input markdown file')
    parser.add_argument('output_json', help='Path to output JSON file')
    parser.add_argument('--id', help='Document ID (generated if not provided)')
    parser.add_argument('--name', help='Document name (uses filename if not provided)')
    parser.add_argument('--document-type', help='Document type (e.g., Financial Report)')
    parser.add_argument('--document-format', help='Document format (e.g., pdf, xlsx, docx, md)')
    parser.add_argument('--document-year', type=int, help='Document year (e.g., 2023, 2024)')
    parser.add_argument('--document-quarter', help='Document quarter (e.g., Q1, Q2, Q3, Q4)')
    parser.add_argument('--metadata', help='Additional metadata as JSON string')
    parser.add_argument('--model', help='Embedding model name', default='intfloat/multilingual-e5-base')
    
    args = parser.parse_args()
    
    # Parse metadata if provided
    metadata = {}
    if args.metadata:
        try:
            metadata = json.loads(args.metadata)
        except json.JSONDecodeError:
            logger.error("Invalid metadata JSON")
            return
    
    # Create chunker and process
    chunker = MarkdownChunker(model_name=args.model)
    num_chunks = await chunker.process_markdown_file(
        markdown_file_path=args.input_md,
        output_json_path=args.output_json,
        id=args.id,
        name=args.name,
        # document_type=args.document_type,
        # document_format=args.document_format,
        # document_year=args.document_year,
        # document_quarter=args.document_quarter,
        # metadata=metadata
    )
    
    if num_chunks > 0:
        logger.info(f"Successfully created {num_chunks} chunks")
    else:
       logger.error("Failed to create chunks")


if __name__ == '__main__':
    asyncio.run(main())
