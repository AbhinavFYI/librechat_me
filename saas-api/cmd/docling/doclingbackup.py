"""
Usage: python document_process.py <input_file_or_url> <output_json_path> --name <name>
"""
import os
import sys
import asyncio
import argparse
import logging
from pathlib import Path
from typing import Optional
import tempfile
import re
import json
import base64
import numpy as np
import easyocr
from io import BytesIO
from PIL import Image
from chunking import MarkdownChunker

# Import from new md_converter architecture
from md_converter import (
    convert_single_async,
    configure_global_settings
)


logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s"
)
logger = logging.getLogger(__name__)


async def extract_and_ocr_images(markdown_path: str, languages: list = ['en'], max_concurrent: int = 4) -> list:
    """
    Extract base64 images from markdown and perform OCR in parallel.
    Returns list of dicts with extracted text.
    
    Args:
        markdown_path: Path to markdown file
        languages: List of language codes for OCR
        max_concurrent: Maximum number of concurrent OCR operations (default: 4)
    """
    # Read markdown file
    with open(markdown_path, 'r', encoding='utf-8') as f:
        markdown_content = f.read()
    
    # Extract base64 images using regex
    pattern = re.compile(r'!\[[^\]]*\]\(data:image/([^;]+);base64,([^\)]+)\)', re.DOTALL)
    matches = pattern.findall(markdown_content)
    
    if not matches:
        logger.info("No embedded images found in markdown")
        return []
    
    logger.info(f"Found {len(matches)} embedded images, initializing OCR...")
    
    # Initialize EasyOCR reader (shared across all tasks)
    reader = easyocr.Reader(languages, gpu=False)
    
    async def process_single_image(idx: int, img_format: str, base64_data: str):
        """Process a single image with OCR"""
        try:
            # Decode base64 to image (run in executor to avoid blocking)
            loop = asyncio.get_event_loop()
            
            def decode_and_ocr():
                image_data = base64.b64decode(base64_data.strip())
                image = Image.open(BytesIO(image_data))
                img_array = np.array(image)
                return reader.readtext(img_array)
            
            # Run OCR in thread pool to avoid blocking
            result = await loop.run_in_executor(None, decode_and_ocr)
            
            if result:
                text_lines = [line[1] for line in result]
                extracted_text = '\n'.join(text_lines)
                
                if extracted_text.strip():
                    logger.info(f"OCR processed image {idx}/{len(matches)} - {len(extracted_text)} chars")
                    return {
                        'image_index': idx,
                        'extracted_text': extracted_text.strip(),
                        'image_format': img_format
                    }
            
        except Exception as e:
            logger.warning(f"OCR failed for image {idx}: {e}")
        
        return None
    
    # Create semaphore for concurrency control
    semaphore = asyncio.Semaphore(max_concurrent)
    
    async def process_with_semaphore(idx, img_format, base64_data):
        async with semaphore:
            return await process_single_image(idx, img_format, base64_data)
    
    # Process all images concurrently
    tasks = [
        process_with_semaphore(idx, img_format, base64_data)
        for idx, (img_format, base64_data) in enumerate(matches, 1)
    ]
    
    results = await asyncio.gather(*tasks)
    
    # Filter out None results
    ocr_results = [r for r in results if r is not None]
    
    logger.info(f"OCR completed: {len(ocr_results)} images with text")
    return ocr_results


class DocumentPipeline:
    """
    End-to-end document processing pipeline.
    Converts documents to markdown and then chunks them.
    """
    
    def __init__(
        self,
        embedding_model: str = "intfloat/multilingual-e5-base",
        temp_dir: Optional[str] = None,
        ocr_enabled: bool = True,
        queue_size: int = 20,
        ocr_batch: int = 12,
        layout_batch: int = 12,
        table_batch: int = 12,
        ocr_lang: list = None
    ):
        """
        Initialize the pipeline.
        
        Args:
            embedding_model: Name of the embedding model to use
            temp_dir: Directory for temporary files (created if not provided)
            ocr_enabled: Enable OCR for scanned documents
            queue_size: Pipeline queue size for backpressure control
            ocr_batch: OCR batch size
            layout_batch: Layout analysis batch size
            table_batch: Table structure batch size
            ocr_lang: OCR languages (default: ["en"])
        """
        self.chunker = MarkdownChunker(model_name=embedding_model)
        self.temp_dir = temp_dir or tempfile.mkdtemp(prefix="doc_pipeline_")
        self.cleanup_temp = temp_dir is None  # Only cleanup if we created it
        
        # Store converter configuration for markdown generation
        self.converter_config = {
            'ocr_enabled': ocr_enabled,
            'queue_size': queue_size,
            'ocr_batch': ocr_batch,
            'layout_batch': layout_batch,
            'table_batch': table_batch,
            'ocr_lang': ocr_lang or ['en']
        }
        
        # Configure global Docling settings
        configure_global_settings(
            doc_batch_size=20,
            page_batch_size=10,
            elements_batch_size=48
        )
        
        logger.info(f"Pipeline initialized with temp directory: {self.temp_dir}")
        logger.info(f"Converter config: OCR={ocr_enabled}, queue={queue_size}, batches=[{ocr_batch},{layout_batch},{table_batch}]")
    
    async def process(
        self,
        input_source: str,
        output_json_path: str,
        id: Optional[str] = None,
        name: Optional[str] = None,
        # document_type: Optional[str] = None,
        # document_format: Optional[str] = None,
        # document_year: Optional[int] = None,
        # document_quarter: Optional[str] = None,
        # metadata: Optional[dict] = None,
        keep_markdown: bool = False,
        ocr: bool = False,
        ocr_languages: list = None,
        ocr_concurrent: int = 4
    ) -> dict:
        """
        Process a document through the complete pipeline.
        
        Args:
            input_source: Path to file or URL
            output_json_path: Where to save the chunked JSON output
            id: Optional document ID
            name: Optional document name (uses filename if not provided)
            document_type: Document type (e.g., 'Financial Report', 'Annual Report')
            document_format: Original document format (auto-detected if not provided)
            document_year: Document year
            document_quarter: Document quarter (Q1, Q2, Q3, Q4)
            metadata: Additional metadata dictionary
            keep_markdown: If True, keep the intermediate markdown file
            
        Returns:
            dict: Processing statistics
        """
        markdown_path = None
        
        try:
            # Check if input is already markdown
            input_path = Path(input_source)
            is_already_markdown = input_path.suffix.lower() in ['.md', '.markdown']
            
            if is_already_markdown:
                # Skip conversion, use the file directly
                logger.info(f"Input is already markdown, skipping conversion step")
                markdown_path = input_path
            else:
                # Step 1: Convert to markdown using new async architecture
                logger.info(f"Step 1/2: Converting '{input_source}' to markdown...")
                temp_output_dir = Path(self.temp_dir)
                
                # Use the new async converter
                markdown_path = await convert_single_async(
                    input_source,
                    temp_output_dir,
                    self.converter_config
                )
                
                if markdown_path is None:
                    raise RuntimeError(f"Failed to convert '{input_source}' to markdown")
                
                logger.info(f"Markdown created: {markdown_path}")
            
            # Auto-detect document format if not provided
            # if not document_format:
            #     input_path = Path(input_source)
            #     document_format = input_path.suffix.lstrip('.').lower() if input_path.suffix else 'unknown'
            
            # # Auto-detect document name if not provided
            # if not document_name:
            #     document_name = Path(input_source).name
            
            # Step 2: Chunk the markdown
            step_num = "Step 1/2" if is_already_markdown else "Step 2/3"
            logger.info(f"{step_num}: Chunking markdown file...")
            
            # Handle output path - if it's a directory, create a filename (same logic as chunker)
            actual_output_path = output_json_path
            if os.path.isdir(output_json_path):
                input_basename = os.path.basename(str(markdown_path))
                input_name_without_ext = os.path.splitext(input_basename)[0]
                output_filename = f"{input_name_without_ext}_chunks.json"
                actual_output_path = os.path.join(output_json_path, output_filename)
            
            num_chunks = await self.chunker.process_markdown_file(
                markdown_file_path=str(markdown_path),
                output_json_path=output_json_path,
                id=id,
                name=name,
                # document_type=document_type,
                # document_format=document_format,
                # document_year=document_year,
                # document_quarter=document_quarter,
                # metadata=metadata
            )
            
            logger.info(f"Chunking complete: {num_chunks} chunks created")
            
            # Step 3: OCR processing (if enabled)
            num_ocr_chunks = 0
            if ocr:
                step_num = "Step 2/2" if is_already_markdown else "Step 3/3"
                logger.info(f"{step_num}: Processing images with OCR...")
                
                ocr_langs = ocr_languages or ['en']
                ocr_results = await extract_and_ocr_images(str(markdown_path), languages=ocr_langs, max_concurrent=ocr_concurrent)
                
                if ocr_results:
                    # Load existing chunks using the actual file path
                    with open(actual_output_path, 'r', encoding='utf-8') as f:
                        data = json.load(f)
                    
                    # Create chunks for OCR results
                    for ocr_item in ocr_results:
                        chunk = {
                            'id': f"{id or 'doc'}_ocr_{ocr_item['image_index']}",
                            'text': ocr_item['extracted_text'],
                            'metadata': {
                                'id': id,
                                'name': name,
                                'type': 'image_ocr',
                                'image_index': ocr_item['image_index'],
                                'image_format': ocr_item['image_format']
                            }
                        }
                        data['chunks'].append(chunk)
                        num_ocr_chunks += 1
                    
                    # Save updated chunks using the actual file path
                    with open(actual_output_path, 'w', encoding='utf-8') as f:
                        json.dump(data, f, indent=2, ensure_ascii=False)
                    
                    logger.info(f"OCR chunking complete: {num_ocr_chunks} image chunks created")
            
            # Step 3: Cleanup or keep markdown
            if keep_markdown or is_already_markdown:
                logger.info(f"Markdown file preserved: {markdown_path}")
            else:
                if markdown_path and os.path.exists(markdown_path) and not is_already_markdown:
                    os.remove(markdown_path)
                    logger.debug(f"Cleaned up temporary markdown: {markdown_path}")
            
            return {
                "success": True,
                "input_source": input_source,
                "output_json": actual_output_path,
                "markdown_path": str(markdown_path) if keep_markdown else None,
                "num_chunks": num_chunks,
                "num_ocr_chunks": num_ocr_chunks,
                "total_chunks": num_chunks + num_ocr_chunks,
                "name": name,
                # "document_format": document_format
            }
            
        except Exception as e:
            logger.error(f"Pipeline failed: {e}")
            import traceback
            logger.error(traceback.format_exc())
            
            return {
                "success": False,
                "error": str(e),
                "input_source": input_source
            }
        
        finally:
            # Cleanup temp directory if we created it
            if self.cleanup_temp and os.path.exists(self.temp_dir):
                try:
                    import shutil
                    shutil.rmtree(self.temp_dir)
                    logger.debug(f"Cleaned up temp directory: {self.temp_dir}")
                except Exception as e:
                    logger.warning(f"Failed to cleanup temp directory: {e}")
    
    async def process_batch(
        self,
        input_sources: list,
        output_dir: str,
        # document_type: Optional[str] = None,
        # document_year: Optional[int] = None,
        # document_quarter: Optional[str] = None,
        # metadata: Optional[dict] = None,
        max_concurrent: int = 3,
        ocr: bool = False,
        ocr_languages: list = None,
        ocr_concurrent: int = 4
    ) -> list:
        """
        Process multiple documents concurrently.
        
        Args:
            input_sources: List of file paths or URLs
            output_dir: Directory where JSON outputs will be saved
            document_type: Document type for all files
            document_year: Document year for all files
            document_quarter: Document quarter for all files
            metadata: Metadata dictionary for all files
            max_concurrent: Maximum number of concurrent processing tasks
            
        Returns:
            list: List of processing results
        """
        # Ensure output directory exists
        output_path = Path(output_dir)
        output_path.mkdir(parents=True, exist_ok=True)
        
        # Create semaphore for concurrency control
        semaphore = asyncio.Semaphore(max_concurrent)
        
        async def process_one(source):
            async with semaphore:
                # Generate output filename
                input_name = Path(source).stem
                output_json = output_path / f"{input_name}_chunks.json"
                
                logger.info(f"Processing: {source}")
                result = await self.process(
                    input_source=source,
                    output_json_path=str(output_json),
                    # document_type=document_type,
                    # document_year=document_year,
                    # document_quarter=document_quarter,
                    # metadata=metadata
                    ocr=ocr,
                    ocr_languages=ocr_languages,
                    ocr_concurrent=ocr_concurrent
                )
                
                return result
        
        # Process all files concurrently (with max_concurrent limit)
        logger.info(f"Starting batch processing of {len(input_sources)} files (max {max_concurrent} concurrent)...")
        results = await asyncio.gather(*[process_one(source) for source in input_sources])
        
        # Summary
        successful = sum(1 for r in results if r.get("success"))
        failed = len(results) - successful
        logger.info(f"Batch processing complete: {successful} successful, {failed} failed")
        
        return results


async def main():
    """CLI interface for the document processing pipeline"""
    parser = argparse.ArgumentParser(
        description='Document Processing Pipeline - Convert and chunk documents',
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    
    # Input/Output arguments
    parser.add_argument('input', nargs='?', help='Input file path or URL')
    parser.add_argument('output', nargs='?', help='Output JSON file path')
    
    # Batch processing
    parser.add_argument('--batch', nargs='+', help='Process multiple files (requires --output-dir)')
    parser.add_argument('--output-dir', help='Output directory for batch processing')
    parser.add_argument('--max-concurrent', type=int, default=3, help='Max concurrent batch processing (default: 3)')
    
    # Document metadata
    parser.add_argument('--id', help='Document ID (generated if not provided)')
    parser.add_argument('--name', help='Document name (uses filename if not provided)')
    parser.add_argument('--document-type', help='Document type (e.g., Financial Report, Annual Report)')
    parser.add_argument('--document-format', help='Document format (auto-detected if not provided)')
    parser.add_argument('--document-year', type=int, help='Document year (e.g., 2023, 2024, 2025)')
    parser.add_argument('--document-quarter', help='Document quarter (Q1, Q2, Q3, Q4)')
    parser.add_argument('--metadata', help='Additional metadata as JSON string')
    
    # Pipeline options
    parser.add_argument('--model', default='intfloat/multilingual-e5-base', help='Embedding model name')
    parser.add_argument('--keep-markdown', action='store_true', help='Keep intermediate markdown file')
    parser.add_argument('--temp-dir', help='Temporary directory for intermediate files')
    
    # Markdown converter options (Docling OCR during PDF->MD conversion)
    parser.add_argument('--no-docling-ocr', action='store_true', help='Disable Docling OCR during markdown conversion')
    parser.add_argument('--docling-ocr-lang', nargs='+', default=['en'], help='Docling OCR languages (default: en)')
    parser.add_argument('--queue-size', type=int, default=20, help='Pipeline queue size (default: 20)')
    parser.add_argument('--ocr-batch', type=int, default=12, help='OCR batch size (default: 12)')
    parser.add_argument('--layout-batch', type=int, default=12, help='Layout batch size (default: 12)')
    parser.add_argument('--table-batch', type=int, default=12, help='Table batch size (default: 12)')
    
    # Image OCR options (for extracting text from embedded images in markdown)
    parser.add_argument('--ocr', action='store_true', help='Enable OCR for embedded images in markdown (default: False)')
    parser.add_argument('--ocr-languages', nargs='+', default=['en'], help='Image OCR language codes (default: en)')
    parser.add_argument('--ocr-concurrent', type=int, default=4, help='Max concurrent image OCR operations (default: 4)')
    
    args = parser.parse_args()
    
    # Validate arguments
    if args.batch:
        if not args.output_dir:
            parser.error("--batch requires --output-dir")
    else:
        if not args.input or not args.output:
            parser.error("Input and output are required for single file processing")
    
    # Parse metadata if provided
    metadata = {}
    if args.metadata:
        try:
            import json
            metadata = json.loads(args.metadata)
        except json.JSONDecodeError:
            logger.error("Invalid metadata JSON")
            return "Failed"
    
    # Initialize pipeline
    pipeline = DocumentPipeline(
        embedding_model=args.model,
        temp_dir=args.temp_dir,
        ocr_enabled=not args.no_docling_ocr,
        queue_size=args.queue_size,
        ocr_batch=args.ocr_batch,
        layout_batch=args.layout_batch,
        table_batch=args.table_batch,
        ocr_lang=args.docling_ocr_lang
    )
    
    try:
        if args.batch:
            # Batch processing
            results = await pipeline.process_batch(
                input_sources=args.batch,
                output_dir=args.output_dir,
                # document_type=args.document_type,
                # document_year=args.document_year,
                # document_quarter=args.document_quarter,
                # metadata=metadata,
                max_concurrent=args.max_concurrent,
                ocr=args.ocr,
                ocr_languages=args.ocr_languages,
                ocr_concurrent=args.ocr_concurrent if args.ocr else 4
            )
            
            for result in results:
                if result.get("success"):
                    return "Success"
                else:
                    return "Failed"
            
        else:
            # Single file processing
            result = await pipeline.process(
                input_source=args.input,
                output_json_path=args.output,
                id=args.id,
                name=args.name,
                # document_type=args.document_type,
                # document_format=args.document_format,
                # document_year=args.document_year,
                # document_quarter=args.document_quarter,
                # metadata=metadata,
                keep_markdown=args.keep_markdown,
                ocr=args.ocr,
                ocr_languages=args.ocr_languages,
                ocr_concurrent=args.ocr_concurrent if args.ocr else 4
            )
            
            if result.get("success"):
               return "Success"
            else:
                return "Failed"

    except Exception as e:
        logger.error(f"Pipeline error: {e}")
        import traceback
        logger.error(traceback.format_exc())
        return "Failed"


if __name__ == '__main__':
    exit_code = asyncio.run(main())
    logger.info(f"Exit code:{exit_code}")

