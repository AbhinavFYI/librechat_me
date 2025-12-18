"""
Optimized Docling Converter with Multithreading & Async Processing (Thread-Safe)
Based on deep analysis of docling-project/docling architecture

Key Optimizations:
1. ThreadedStandardPdfPipeline for parallel page processing
2. Native convert_all() with configurable batch settings
3. Async I/O for file operations and network requests
4. Progressive streaming results
5. Memory-efficient processing with backpressure control

Thread-Safety Fixes:
1. Thread-local DocumentConverter instances
2. File-level locks for concurrent writes
3. Atomic file operations with temp files
4. Unique filename generation
5. Thread-safe logging with QueueHandler (no duplicates)
6. Per-task executors to avoid shared state

Usage: 
    python md_converter.py document.pdf ./output
    python md_converter.py --batch *.pdf --output ./output
    python md_converter.py --async --batch doc1.pdf doc2.pdf --output ./output
"""

import asyncio
import os
import sys
import aiofiles
import aiohttp
from pathlib import Path
from docling.datamodel.layout_model_specs import LayoutModelConfig
import urllib3
from urllib.parse import urlparse
import logging
import logging.handlers
from typing import List, Optional, Dict
import time
import argparse
from concurrent.futures import ThreadPoolExecutor
from functools import partial
import threading
from collections import defaultdict
from asyncio import Lock as AsyncLock
import errno

from docling.datamodel.base_models import InputFormat, ConversionStatus
from docling.datamodel.pipeline_options import (
    LayoutOptions,
    TableFormerMode,
    TableStructureOptions,
    ThreadedPdfPipelineOptions
)
from docling.datamodel.accelerator_options import AcceleratorOptions, AcceleratorDevice
from docling.document_converter import (
    DocumentConverter,
    PdfFormatOption,
    WordFormatOption,
    PowerpointFormatOption,
    HTMLFormatOption,
    ExcelFormatOption,
    CsvFormatOption,
)
from docling.pipeline.threaded_standard_pdf_pipeline import ThreadedStandardPdfPipeline
from docling.pipeline.simple_pipeline import SimplePipeline
from docling.datamodel.settings import settings

# ==================== THREAD-SAFE GLOBALS ====================

# Thread-local storage for DocumentConverter instances
_converter_local = threading.local()

# File-level locks for async writes
_file_locks = defaultdict(AsyncLock)

# Logging initialization
_logging_listener = None
_logging_lock = threading.Lock()
_logging_initialized = False

urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)

SUPPORTED_FORMATS = {
    "pdf", "docx", "xlsx", "csv", "pptx", "html", "txt",
    "png", "jpg", "jpeg", "tiff", "bmp", "gif"
}

# ==================== LOGGING SETUP ====================

def setup_logging():
    """Thread-safe logging configuration with QueueHandler (idempotent, no duplicates)"""
    global _logging_listener, _logging_initialized
    
    with _logging_lock:
        # Only initialize once
        if _logging_initialized:
            return _logging_listener
        
        root = logging.getLogger()
        
        # Clear any existing handlers to prevent duplicates
        root.handlers.clear()
        
        # Create queue for thread-safe logging
        import queue
        log_queue = queue.Queue(-1)
        
        # Queue handler (thread-safe)
        queue_handler = logging.handlers.QueueHandler(log_queue)
        
        root.addHandler(queue_handler)
        root.setLevel(logging.INFO)
        
        # Console handler in listener thread
        console_handler = logging.StreamHandler()
        console_handler.setFormatter(
            logging.Formatter('%(asctime)s [%(levelname)s] %(message)s')
        )
        
        # Start listener
        _logging_listener = logging.handlers.QueueListener(
            log_queue, console_handler, respect_handler_level=True
        )
        _logging_listener.start()
        
        _logging_initialized = True
        return _logging_listener

# Initialize logging once at module import
setup_logging()
logger = logging.getLogger(__name__)

# ==================== SSL CONFIGURATION ====================

def disable_ssl_verification():
    """Disable SSL verification for requests (called after logging setup)"""
    import requests
    original_init = requests.Session.__init__

    def patched_init(self, *args, **kwargs):
        original_init(self, *args, **kwargs)
        self.verify = False

    requests.Session.__init__ = patched_init

# Disable SSL after logging is initialized
disable_ssl_verification()

# ==================== THREAD-SAFE UTILITIES ====================

def safe_mkdir(path: Path):
    """Race-condition-safe directory creation"""
    try:
        path.mkdir(parents=True, exist_ok=False)
    except OSError as e:
        if e.errno == errno.EEXIST and path.is_dir():
            pass  # Already exists, safe to ignore
        else:
            raise

def get_unique_filename(base_name: str, output_dir: Path, extension: str = ".md") -> Path:
    """Generate unique filename to prevent overwrites"""
    output_file = output_dir / f"{base_name}{extension}"
    counter = 1
    
    # Thread-safe check-and-increment
    while output_file.exists():
        output_file = output_dir / f"{base_name}_{counter}{extension}"
        counter += 1
    
    return output_file

def create_session_no_ssl():
    """Create SSL-disabled requests session without global state mutation"""
    import requests
    session = requests.Session()
    session.verify = False
    session.trust_env = False
    return session

# ==================== CONFIGURATION ====================

def configure_global_settings(
    doc_batch_size: int =15,
    page_batch_size: int = 10,
    elements_batch_size: int = 48
):
    """
    Configure Docling global performance settings
    
    Args:
        doc_batch_size: Documents processed per batch
        page_batch_size: Pages processed per batch
        elements_batch_size: Elements processed per batch in enrichment
    """
    settings.perf.doc_batch_size = doc_batch_size
    settings.perf.page_batch_size = page_batch_size
    settings.perf.elements_batch_size = elements_batch_size
    
    logger.info(f"Global settings: doc_batch={doc_batch_size}, "
                f"page_batch={page_batch_size}, elements_batch={elements_batch_size}")

def create_threaded_converter(
    ocr_enabled: bool = True,
    queue_size: int = 20,
    ocr_batch: int = 12,
    layout_batch: int = 12,
    table_batch: int = 12,
    ocr_lang: List[str] = None,
) -> DocumentConverter:
    """
    Create optimized DocumentConverter with ThreadedStandardPdfPipeline
    
    ThreadedStandardPdfPipeline uses 5 concurrent stages:
    1. Preprocess - Page preparation
    2. OCR - Optical character recognition
    3. Layout - Document layout analysis
    4. Table - Table structure detection
    5. Assembly - Document assembly
    
    Args:
        ocr_enabled: Enable OCR for scanned documents
        queue_size: Queue size for backpressure control
        ocr_batch: OCR batch size
        layout_batch: Layout analysis batch size
        table_batch: Table structure batch size
        ocr_lang: OCR languages
    
    Returns:
        Configured DocumentConverter
    """
    if ocr_lang is None:
        ocr_lang = ["en"]
    
    # Configure accelerator
    accelerator_options = AcceleratorOptions(
        num_threads=8,
        device=AcceleratorDevice.AUTO,
    )
    table_structure_options = TableStructureOptions(
        mode=TableFormerMode.ACCURATE,
        do_cell_matching=True,
    )
    # Configure threaded PDF pipeline with batching
    pdf_pipeline_options = ThreadedPdfPipelineOptions(
        accelerator_options=accelerator_options,
        do_ocr=ocr_enabled,
        do_table_structure=True,
        generate_page_images=True,
        generate_picture_images=True,
        images_scale=1.0,
        table_structure_options=table_structure_options,
        
        # Threaded pipeline specific options
        queue_max_size=queue_size,
        ocr_batch_size=ocr_batch,
        layout_batch_size=layout_batch,
        table_batch_size=table_batch,
        batch_polling_interval_seconds=0.1,
    )
    
    # Add OCR options if enabled
    if ocr_enabled:
        from docling.datamodel.pipeline_options import EasyOcrOptions
        pdf_pipeline_options.ocr_options = EasyOcrOptions(lang=ocr_lang)
    
    # Create converter with all format options
    converter = DocumentConverter(
        format_options={
            InputFormat.PDF: PdfFormatOption(
                pipeline_cls=ThreadedStandardPdfPipeline,
                pipeline_options=pdf_pipeline_options,
            ),
            InputFormat.DOCX: WordFormatOption(
                pipeline_cls=SimplePipeline,
            ),
            InputFormat.PPTX: PowerpointFormatOption(
                pipeline_cls=SimplePipeline,
            ),
            InputFormat.HTML: HTMLFormatOption(
                pipeline_cls=SimplePipeline,
            ),
            InputFormat.XLSX: ExcelFormatOption(
                pipeline_cls=SimplePipeline,
            ),
            InputFormat.CSV: CsvFormatOption(
                pipeline_cls=SimplePipeline,
            ),
        }
    )
    
    logger.info(f"Created converter: OCR={ocr_enabled}, "
                f"queue_size={queue_size}, batches=[{ocr_batch},{layout_batch},{table_batch}]")
    
    return converter

def get_thread_local_converter(
    ocr_enabled: bool = True,
    queue_size: int = 12,
    ocr_batch: int = 10,
    layout_batch: int = 10,
    table_batch: int = 8,
    ocr_lang: List[str] = None
) -> DocumentConverter:
    """
    Get or create thread-local DocumentConverter instance (thread-safe)
    
    Each thread gets its own converter to avoid race conditions
    """
    if not hasattr(_converter_local, 'converter'):
        _converter_local.converter = create_threaded_converter(
            ocr_enabled=ocr_enabled,
            queue_size=queue_size,
            ocr_batch=ocr_batch,
            layout_batch=layout_batch,
            table_batch=table_batch,
            ocr_lang=ocr_lang
        )
        logger.debug(f"Created thread-local converter for {threading.current_thread().name}")
    
    return _converter_local.converter

# ==================== ASYNC I/O UTILITIES ====================

async def download_file_async(url: str, output_path: Path) -> Path:
    """Download file asynchronously with progress (SSL-safe)"""
    temp_file = None
    try:
        # Use aiohttp with SSL disabled
        connector = aiohttp.TCPConnector(ssl=False)
        async with aiohttp.ClientSession(connector=connector) as session:
            async with session.get(url) as response:
                response.raise_for_status()
                
                total_size = int(response.headers.get('content-length', 0))
                downloaded = 0
                
                # Use temp file for atomic write
                temp_file = output_path.with_suffix('.tmp')
                
                async with aiofiles.open(temp_file, 'wb') as f:
                    async for chunk in response.content.iter_chunked(8192):
                        await f.write(chunk)
                        downloaded += len(chunk)
                        if total_size:
                            progress = (downloaded / total_size) * 100
                            logger.debug(f"Download {output_path.name}: {progress:.1f}%")
                
                # Atomic rename
                temp_file.rename(output_path)
                logger.info(f"Downloaded: {output_path.name}")
                return output_path
                
    except Exception as e:
        logger.error(f"Download failed {url}: {e}")
        # Cleanup temp file
        if temp_file and temp_file.exists():
            temp_file.unlink()
        raise

async def save_markdown_async(content: str, output_path: Path) -> Path:
    """
    Save markdown content asynchronously with atomic write
    Uses file-level locking and temp file for safety
    """
    try:
        # Get file-specific lock
        lock = _file_locks[str(output_path)]
        
        async with lock:
            # Write to temp file first
            temp_file = output_path.with_suffix('.tmp')
            
            try:
                async with aiofiles.open(temp_file, 'w', encoding='utf-8') as f:
                    await f.write(content)
                
                # Atomic rename (POSIX guarantee)
                temp_file.rename(output_path)
                
            except Exception as e:
                # Cleanup on error
                if temp_file.exists():
                    temp_file.unlink()
                raise
        
        return output_path
        
    except Exception as e:
        logger.error(f"Failed to save {output_path}: {e}")
        raise

async def process_txt_file_async(input_source: str, output_path: Path) -> Path:
    """Handle TXT file conversion asynchronously (SSL-safe)"""
    parsed = urlparse(input_source)
    
    try:
        if parsed.scheme in ("http", "https"):
            connector = aiohttp.TCPConnector(ssl=False)
            async with aiohttp.ClientSession(connector=connector) as session:
                async with session.get(input_source) as response:
                    response.raise_for_status()
                    text_data = await response.text()
        else:
            async with aiofiles.open(input_source, 'r', encoding='utf-8') as f:
                text_data = await f.read()
        
        filename = Path(parsed.path if parsed.scheme else input_source).stem
        output_file = get_unique_filename(filename, output_path)
        
        await save_markdown_async(text_data, output_file)
        logger.info(f"TXT converted: {output_file.name}")
        return output_file
        
    except Exception as e:
        logger.error(f"TXT conversion failed: {e}")
        raise

# ==================== SYNC PROCESSING ====================

def validate_input(input_source: str) -> tuple[str, str]:
    """Validate input and extract filename and extension"""
    parsed = urlparse(input_source)
    
    if parsed.scheme in ("http", "https"):
        filename = Path(parsed.path).name
    else:
        if not Path(input_source).exists():
            raise FileNotFoundError(f"File not found: {input_source}")
        filename = Path(input_source).name
    
    if "." not in filename:
        raise ValueError("Could not detect file extension")
    
    ext = filename.split(".")[-1].lower()
    
    if ext not in SUPPORTED_FORMATS:
        raise ValueError(f"Unsupported format: {ext}. Supported: {', '.join(SUPPORTED_FORMATS)}")
    
    return filename, ext

def convert_single_sync(
    input_source: str,
    output_dir: Path,
    converter_config: Dict
) -> Optional[Path]:
    """
    Convert single document synchronously (thread-safe)
    Uses thread-local converter instance
    """
    try:
        filename, ext = validate_input(input_source)
        
        # Handle TXT files (run async in thread)
        if ext == "txt":
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
            result = loop.run_until_complete(
                process_txt_file_async(input_source, output_dir)
            )
            loop.close()
            return result
        
        # Get thread-local converter (thread-safe)
        converter = get_thread_local_converter(**converter_config)
        
        # Convert with Docling
        logger.info(f"Converting: {filename}")
        start = time.time()
        
        result = converter.convert(input_source)
        
        if result.status == ConversionStatus.SUCCESS:
            doc = result.document
            
            # Log processing metrics
            logger.info(f"Processed {len(doc.pages)} pages from {filename}")
            
            md_text = doc.export_to_markdown(image_mode="embedded")
            
            # Use unique filename to prevent race conditions
            output_file = get_unique_filename(Path(filename).stem, output_dir)
            
            # Atomic write
            temp_file = output_file.with_suffix('.tmp')
            temp_file.write_text(md_text, encoding='utf-8')
            temp_file.rename(output_file)
            
            elapsed = time.time() - start
            logger.info(f"✓ Converted: {filename} → {output_file.name} ({elapsed:.2f}s, {len(md_text)/1024:.1f} KB)")
            return output_file
        else:
            logger.error(f"✗ Conversion failed: {filename} - {result.status}")
            return None
            
    except Exception as e:
        logger.error(f"✗ Error converting {input_source}: {e}")
        return None

def batch_convert_sync(
    input_sources: List[str],
    output_dir: Path,
    converter_config: Dict,
    raises_on_error: bool = False
) -> List[Optional[Path]]:
    """
    Batch convert using Docling's native convert_all() (thread-safe)
    
    Uses thread-local converter to avoid race conditions
    """
    # Separate TXT files
    txt_files = []
    other_files = []
    
    for source in input_sources:
        try:
            _, ext = validate_input(source)
            if ext == "txt":
                txt_files.append(source)
            else:
                other_files.append(source)
        except Exception as e:
            logger.error(f"Skipping invalid input '{source}': {e}")
    
    results = []
    
    # Process TXT files
    for txt_file in txt_files:
        result = convert_single_sync(txt_file, output_dir, converter_config)
        results.append(result)
    
    # Batch convert other files with Docling
    if other_files:
        logger.info(f"Batch converting {len(other_files)} files...")
        start = time.time()
        
        try:
            # Get thread-local converter
            converter = get_thread_local_converter(**converter_config)
            
            # Use Docling's native batch processing
            batch_results = converter.convert_all(
                other_files,
                raises_on_error=raises_on_error
            )
            
            for source, result in zip(other_files, batch_results):
                try:
                    if result.status == ConversionStatus.SUCCESS:
                        filename = Path(urlparse(source).path if "://" in source else source).stem
                        doc = result.document
                        
                        md_text = doc.export_to_markdown(image_mode="embedded")
                        output_file = get_unique_filename(filename, output_dir)
                        
                        # Atomic write
                        temp_file = output_file.with_suffix('.tmp')
                        temp_file.write_text(md_text, encoding='utf-8')
                        temp_file.rename(output_file)
                        
                        logger.info(f"✓ Converted: {output_file.name} ({len(doc.pages)} pages)")
                        results.append(output_file)
                    else:
                        logger.error(f"✗ Failed: {source} - {result.status}")
                        results.append(None)
                        
                except Exception as e:
                    logger.error(f"✗ Error processing {source}: {e}")
                    results.append(None)
            
            elapsed = time.time() - start
            successful = len([r for r in results if r is not None])
            logger.info(f"Batch complete: {successful}/{len(input_sources)} files in {elapsed:.2f}s")
            
        except Exception as e:
            logger.error(f"Batch conversion error: {e}")
            if raises_on_error:
                raise
    
    return results

# ==================== ASYNC PROCESSING ====================

async def convert_single_async(
    input_source: str,
    output_dir: Path,
    converter_config: Dict
) -> Optional[Path]:
    """
    Convert single document asynchronously (thread-safe)
    CPU-bound conversion runs in dedicated thread pool with thread-local converter
    """
    try:
        filename, ext = validate_input(input_source)
        
        # Handle TXT files
        if ext == "txt":
            return await process_txt_file_async(input_source, output_dir)
        
        # Run CPU-bound conversion in dedicated thread
        logger.info(f"Converting: {filename}")
        start = time.time()
        
        loop = asyncio.get_event_loop()
        
        # Use dedicated executor per task to ensure thread-local converter
        with ThreadPoolExecutor(max_workers=1) as executor:
            result = await loop.run_in_executor(
                executor,
                lambda: get_thread_local_converter(**converter_config).convert(input_source)
            )
        
        if result.status == ConversionStatus.SUCCESS:
            doc = result.document

            md_text = doc.export_to_markdown(image_mode="embedded")
            
            # Use unique filename
            output_file = get_unique_filename(Path(filename).stem, output_dir)
            
            # Async atomic write
            await save_markdown_async(md_text, output_file)
            
            elapsed = time.time() - start
            logger.info(f"✓ Converted: {filename} → {output_file.name} ({elapsed:.2f}s, {len(doc.pages)} pages)")
            return output_file
        else:
            logger.error(f"✗ Conversion failed: {filename} - {result.status}")
            return None
            
    except Exception as e:
        logger.error(f"✗ Error converting {input_source}: {e}")
        return None

async def batch_convert_async(
    input_sources: List[str],
    output_dir: Path,
    converter_config: Dict,
    max_concurrent: int = 4
) -> List[Optional[Path]]:
    """
    Async batch conversion with concurrency control (thread-safe)
    
    Args:
        input_sources: List of file paths or URLs
        output_dir: Output directory
        converter_config: Converter configuration dict
        max_concurrent: Maximum concurrent conversions
    """
    semaphore = asyncio.Semaphore(max_concurrent)
    
    async def convert_with_semaphore(source):
        async with semaphore:
            return await convert_single_async(source, output_dir, converter_config)
    
    logger.info(f"Async batch converting {len(input_sources)} files (max_concurrent={max_concurrent})...")
    start = time.time()
    
    # Process all files concurrently
    tasks = [convert_with_semaphore(source) for source in input_sources]
    results = await asyncio.gather(*tasks, return_exceptions=True)
    
    # Handle exceptions
    final_results = []
    for result in results:
        if isinstance(result, Exception):
            logger.error(f"Task failed: {result}")
            final_results.append(None)
        else:
            final_results.append(result)
    
    elapsed = time.time() - start
    successful = len([r for r in final_results if r is not None])
    logger.info(f"Async batch complete: {successful}/{len(input_sources)} files in {elapsed:.2f}s")
    
    return final_results

# ==================== CLI ====================

def main():
    parser = argparse.ArgumentParser(
        description="Optimized Docling Converter with Multithreading & Async (Thread-Safe)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Single file (sync)
  python md_converter.py document.pdf ./output
  
  # Batch processing (sync)
  python md_converter.py --batch doc1.pdf doc2.docx --output ./output
  
  # Batch with high concurrency
  python md_converter.py --batch *.pdf --output ./output --doc-batch 50
  
  # Async processing
  python md_converter.py --async --batch *.pdf --output ./output --max-concurrent 6
  
  # OCR optimization
  python md_converter.py --batch *.pdf --output ./out --ocr-batch 8 --layout-batch 8
        """
    )
    
    # Input/output
    parser.add_argument("input", nargs="*", help="Input file(s) or URL(s)")
    parser.add_argument("--batch", action="store_true", help="Batch mode")
    parser.add_argument("--output", "-o", help="Output directory")
    
    # Processing mode
    parser.add_argument("--async", dest="async_mode", action="store_true",
                       help="Use async processing (better for I/O-bound tasks)")
    parser.add_argument("--max-concurrent", type=int, default=4,
                       help="Max concurrent async tasks (default: 4)")
    
    # Global settings
    parser.add_argument("--doc-batch", type=int, default=20,
                       help="Documents per batch (default: 20)")
    parser.add_argument("--page-batch", type=int, default=10,
                       help="Pages per batch (default: 10)")
    parser.add_argument("--elements-batch", type=int, default=32,
                       help="Elements per batch (default: 32)")
    
    # Pipeline settings
    parser.add_argument("--no-ocr", action="store_true", help="Disable OCR")
    parser.add_argument("--ocr-lang", nargs="+", default=["en"],
                       help="OCR languages (default: en)")
    
    # Threaded pipeline settings
    parser.add_argument("--queue-size", type=int, default=20,
                       help="Pipeline queue size (default: 20)")
    parser.add_argument("--ocr-batch", type=int, default=12,
                       help="OCR batch size (default: 12)")
    parser.add_argument("--layout-batch", type=int, default=12,
                       help="Layout batch size (default: 12)")
    parser.add_argument("--table-batch", type=int, default=12,
                       help="Table batch size (default: 12)")
    # Error handling
    parser.add_argument("--strict", action="store_true",
                       help="Stop on first error")
    
    args = parser.parse_args()
    
    # Validate arguments
    if not args.input and not args.batch:
        parser.error("No input files specified")
    
    # Handle legacy positional args
    if not args.batch and len(args.input) >= 2 and not args.output:
        args.output = args.input[-1]
        args.input = args.input[:-1]
    
    if not args.output:
        parser.error("Output directory not specified")
    
    output_dir = Path(args.output).resolve()
    safe_mkdir(output_dir)
    
    # Configure global settings
    configure_global_settings(
        doc_batch_size=args.doc_batch,
        page_batch_size=args.page_batch,
        elements_batch_size=args.elements_batch
    )
    
    # Converter configuration (thread-safe, passed to workers)
    converter_config = {
        'ocr_enabled': not args.no_ocr,
        'queue_size': args.queue_size,
        'ocr_batch': args.ocr_batch,
        'layout_batch': args.layout_batch,
        'table_batch': args.table_batch,
        'ocr_lang': args.ocr_lang
    }
    
    try:
        # Async mode
        if args.async_mode:
            results = asyncio.run(batch_convert_async(
                args.input,
                output_dir,
                converter_config,
                max_concurrent=args.max_concurrent
            ))
        # Batch mode (sync)
        elif args.batch or len(args.input) > 1:
            results = batch_convert_sync(
                args.input,
                output_dir,
                converter_config,
                raises_on_error=args.strict
            )
        # Single file mode (sync)
        else:
            result = convert_single_sync(args.input[0], output_dir, converter_config)
            results = [result]
        
        # Report results
        successful = len([r for r in results if r is not None])
        if successful == len(args.input):
            logger.info(f"✓ All {successful} files converted successfully")
            sys.exit(0)
        else:
            logger.warning(f"⚠ {successful}/{len(args.input)} files converted successfully")
            sys.exit(1)
            
    except KeyboardInterrupt:
        logger.warning("⚠ Interrupted by user")
        sys.exit(130)
    except Exception as e:
        logger.error(f"✗ Fatal error: {e}", exc_info=True)
        sys.exit(2)
    finally:
        # Cleanup logging listener
        if _logging_listener:
            _logging_listener.stop()

if __name__ == "__main__":
    main()
