"""
Document Chunking Pipeline with Docling

This module provides intelligent document chunking using Docling's HybridChunker
with support for multiple document formats (PDF, DOCX, TXT, JSON, etc.) and OCR capabilities.

Processing methods:
- Most formats: Converted to DoclingDocument, then chunked with HybridChunker
- JSON files: Content read directly (no DoclingDocument conversion)
  - JSON objects ({...}): Single chunk with entire object
  - JSON arrays ([...]): Each array item becomes a separate chunk
  - Preserves original JSON structure in output

All chunks maintain consistent _chunks.json structure with metadata:
- chunk_id, content, content_type, page_number, section_title, chunk_index

Usage:
    python document_process.py <input> <output> [options]
"""
import os
import logging
# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
    datefmt='%Y-%m-%d %H:%M:%S'
)
logger = logging.getLogger(__name__)

if os.uname().machine == 'aarch64':
    os.environ["OMP_NUM_THREADS"] = "8"
    os.environ["MKL_NUM_THREADS"] = "8"
    os.environ["NUMEXPR_NUM_THREADS"] = "8"
    os.environ["OPENBLAS_NUM_THREADS"] = "8"
    os.environ["TOKENIZERS_PARALLELISM"] = "false"
    logger.info("Detected ARM/Graviton3 - Applied m7g.2xlarge optimizations")
import json
import threading
import tempfile
from docling.pipeline.standard_pdf_pipeline import StandardPdfPipeline
import urllib3
from pathlib import Path
from typing import List, Optional, Dict, Any
from dataclasses import dataclass, asdict
from urllib.parse import urlparse
import requests
from docling.document_converter import DocumentConverter,ImageFormatOption
from docling.chunking import HybridChunker
from docling.pipeline.simple_pipeline import SimplePipeline
from docling_core.transforms.chunker.tokenizer.huggingface import HuggingFaceTokenizer
from docling_core.transforms.serializer.markdown import MarkdownTableSerializer
from docling_core.transforms.chunker.hierarchical_chunker import (
    ChunkingDocSerializer,
    ChunkingSerializerProvider
)
from transformers import AutoTokenizer
from docling.pipeline.threaded_standard_pdf_pipeline import ThreadedStandardPdfPipeline
from docling.datamodel.pipeline_options import ThreadedPdfPipelineOptions
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import (
    AcceleratorOptions,
    AcceleratorDevice,
    TableStructureOptions,
    TableFormerMode,
)
from docling.document_converter import (
    PdfFormatOption,
    WordFormatOption,
    PowerpointFormatOption,
    HTMLFormatOption,
    ExcelFormatOption,
    CsvFormatOption,
    MarkdownFormatOption,
)


def disable_ssl_verification():
    """Disable SSL verification for requests."""
    import requests
    original_init = requests.Session.__init__

    def patched_init(self, *args, **kwargs):
        original_init(self, *args, **kwargs)
        self.verify = False

    requests.Session.__init__ = patched_init


disable_ssl_verification()

# Suppress SSL warnings for cleaner output
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)





# Thread-local storage for converters
_converter_local = threading.local()

# Supported document formats (all will be chunked using HybridChunker)
SUPPORTED_FORMATS = {
    ".docx", ".dotx", ".docm", ".dotm",
    ".pptx",
    ".pdf",
    ".md",
    ".html", ".htm", ".xhtml",
    ".jpg", ".jpeg", ".png", ".tiff", ".bmp", ".webp",
    ".csv",
    ".xlsx", ".xlsm",
    ".txt",  
    ".json",
}


@dataclass
class Chunk:
    """Represents a single chunk with metadata."""
    chunk_id: str
    content: str
    content_type: str
    page_number: Optional[int] = None
    section_title: Optional[str] = None
    chunk_index: Optional[int] = None


class MDTableSerializerProvider(ChunkingSerializerProvider):
    """Custom provider for markdown table serialization"""
    
    def get_serializer(self, doc):
        return ChunkingDocSerializer(
            doc=doc,
            table_serializer=MarkdownTableSerializer()
        )


def create_threaded_converter(
    ocr_enabled: bool = True,
    queue_size: int = 100,
    ocr_batch: int = 24,
    layout_batch: int = 32,
    table_batch: int = 16,
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
            InputFormat.MD: MarkdownFormatOption(
                pipeline_cls=SimplePipeline,
            ),
            InputFormat.IMAGE: ImageFormatOption(
                pipeline_cls=StandardPdfPipeline,
                pipeline_options=pdf_pipeline_options,
            ),
        }
    )
    
    logger.debug(f"Converter created - OCR: {ocr_enabled}, Queue: {queue_size}, "
                 f"Batches: OCR={ocr_batch}, Layout={layout_batch}, Table={table_batch}")
    
    return converter


def get_thread_local_converter(
    ocr_enabled: bool = True,
    queue_size: int = 100,
    ocr_batch: int = 24,
    layout_batch: int = 32,
    table_batch: int = 16,
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


def download_file_from_url(url: str) -> Optional[str]:
    """
    Download file from URL to temporary location (SSL-safe)
    
    Args:
        url: HTTP/HTTPS URL to download from
        
    Returns:
        Path to downloaded temporary file, or None if failed
    """
    try:
        logger.info(f"Downloading file from URL: {url}")
        
        # Get file extension from URL
        parsed = urlparse(url)
        path = Path(parsed.path)
        ext = path.suffix if path.suffix else '.pdf'
        
        # Download with SSL verification disabled
        response = requests.get(url, verify=False, timeout=60)
        response.raise_for_status()
        
        # Save to temporary file
        temp_file = tempfile.NamedTemporaryFile(delete=False, suffix=ext)
        temp_file.write(response.content)
        temp_file.close()
        
        logger.info(f"Downloaded to temporary file: {temp_file.name}")
        return temp_file.name
        
    except Exception as e:
        logger.error(f"Failed to download file from URL: {e}", exc_info=True)
        return None


def chunk_json_directly(
    input_path: Path,
    source_name: str,
    output_json: str,
    embedding_model: str,
    max_tokens: int,
    temp_file: Optional[str] = None
) -> int:
    """
    Process JSON files directly by reading content and creating chunks without DoclingDocument conversion.
    
    Behavior:
        - JSON objects ({...}): Single chunk with entire content (content_type='json_object')
        - JSON arrays ([...]): Each array item becomes a separate chunk (content_type='json_array_item')
        - Other types: Single chunk (content_type='json')
    
    Output format matches standard _chunks.json structure with:
        - chunk_id, content, content_type, page_number, section_title, chunk_index
    
    Args:
        input_path: Path to JSON file
        source_name: Source file name
        output_json: Output JSON path
        embedding_model: HuggingFace embedding model ID
        max_tokens: Maximum tokens per chunk
        temp_file: Temporary file to clean up (if downloaded)
        
    Returns:
        Number of chunks created
    """
    try:
        # Read JSON content
        logger.info("Reading JSON content")
        with open(input_path, 'r', encoding='utf-8') as f:
            json_content = f.read()
        
        # Load tokenizer
        logger.info(f"Loading tokenizer: {embedding_model}")
        hf_tokenizer = AutoTokenizer.from_pretrained(embedding_model)
        tokenizer = HuggingFaceTokenizer(
            tokenizer=hf_tokenizer,
            max_tokens=max_tokens
        )
        logger.info(f"Tokenizer loaded successfully (max_tokens={max_tokens})")
        
        # Pretty format the JSON for better readability
        try:
            json_data = json.loads(json_content)
            formatted_content = json.dumps(json_data, indent=2, ensure_ascii=False)
        except json.JSONDecodeError:
            logger.warning("Invalid JSON format, using raw content")
            formatted_content = json_content
        
        # Parse JSON to determine structure
        chunks: List[Chunk] = []
        
        # Check if it's an array of objects
        if isinstance(json_data, list):
            # JSON array - each object becomes a chunk
            logger.info(f"JSON is an array with {len(json_data)} items - creating one chunk per item")
            
            for chunk_index, item in enumerate(json_data):
                # Convert each item to formatted JSON string
                item_content = json.dumps(item, indent=2, ensure_ascii=False)
                
                chunk = Chunk(
                    chunk_id=f"{source_name}_chunk_{chunk_index:04d}",
                    content=item_content,
                    content_type='json_array_item',
                    page_number=1,
                    section_title=f"Item {chunk_index + 1}",
                    chunk_index=chunk_index,
                )
                chunks.append(chunk)
            
            logger.info(f"Created {len(chunks)} chunks from JSON array")
        
        elif isinstance(json_data, dict):
            # JSON object - create as single chunk
            logger.info("JSON is an object - creating single chunk with entire content")
            
            chunk = Chunk(
                chunk_id=f"{source_name}_chunk_0000",
                content=formatted_content,
                content_type='json_object',
                page_number=1,
                section_title="JSON Object",
                chunk_index=0,
            )
            chunks.append(chunk)
            logger.info(f"Created single chunk for JSON object")
        
        else:
            # Other JSON types (string, number, etc.) - create as single chunk
            logger.info("JSON is a primitive type - creating single chunk")
            
            chunk = Chunk(
                chunk_id=f"{source_name}_chunk_0000",
                content=formatted_content,
                content_type='json',
                page_number=1,
                section_title="JSON Data",
                chunk_index=0,
            )
            chunks.append(chunk)
        
        logger.info(f"Successfully generated {len(chunks)} chunks from JSON")
        
        # Save chunks to JSON
        logger.info(f"Saving {len(chunks)} chunks to {output_json}")
        output_path = Path(output_json)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        
        json_data = [asdict(chunk) for chunk in chunks]
        
        with open(output_path, 'w', encoding='utf-8') as f:
            json.dump(json_data, f, indent=2, ensure_ascii=False)
        
        logger.info(f"Successfully saved chunks to {output_path}")
        
        # Clean up temporary file if it was downloaded
        if temp_file:
            try:
                Path(temp_file).unlink()
                logger.debug(f"Cleaned up temporary file: {temp_file}")
            except Exception as cleanup_error:
                logger.warning(f"Failed to clean up temporary file: {cleanup_error}")
        
        return len(chunks)
    
    except Exception as e:
        logger.error(f"Failed to process JSON file: {e}", exc_info=True)
        
        # Clean up temporary file on error
        if temp_file:
            try:
                Path(temp_file).unlink()
            except Exception:
                pass
        
        return 0


def chunk_document(
    input_file: str,
    output_json: str,
    embedding_model: str = "intfloat/multilingual-e5-base",
    max_tokens: int = 1024,
    ocr_enabled: bool = True,
    ocr_lang: List[str] = None,
) -> int:
    """
    Convert document to DoclingDocument (with threaded PDF pipeline) and chunk it using HybridChunker.
    
    Special handling:
        - JSON files: Content is read directly and chunked without DoclingDocument conversion
        - Other formats: Converted to DoclingDocument first, then chunked with HybridChunker
    
    Args:
        input_file: Path to input document or URL (any supported format)
        output_json: Path to output JSON file
        embedding_model: HuggingFace embedding model ID
        max_tokens: Maximum tokens per chunk
        ocr_enabled: Enable OCR for PDFs
        ocr_lang: OCR languages (e.g., ["en", "hi"])
        
    Returns:
        Number of chunks created
    """
    
    # Check if input is a URL
    temp_file = None
    temp_md_file = None
    parsed = urlparse(input_file)
    if parsed.scheme in ("http", "https"):
        temp_file = download_file_from_url(input_file)
        if not temp_file:
            logger.error(f"Failed to download file from URL: {input_file}")
            return 0
        input_path = Path(temp_file)
        source_name = Path(parsed.path).name or "downloaded_document"
    else:
        # Validate local input file
        input_path = Path(input_file)
        if not input_path.exists():
            logger.error(f"File not found: {input_file}")
            return 0
        source_name = input_path.name
    
    logger.info(f"Processing document: {source_name}")
    
    # Special handling for JSON files - chunk directly without DoclingDocument conversion
    if input_path.suffix.lower() == '.json':
        logger.info("Detected JSON file - processing directly without DoclingDocument conversion")
        return chunk_json_directly(
            input_path=input_path,
            source_name=source_name,
            output_json=output_json,
            embedding_model=embedding_model,
            max_tokens=max_tokens,
            temp_file=temp_file
        )
    
    # Handle .txt files by converting them to .md temporarily
    # (Docling processes .md files better than plain .txt)
    if input_path.suffix.lower() == '.txt':
        logger.debug("Converting .txt file to .md for processing")
        temp_md_file = tempfile.NamedTemporaryFile(delete=False, suffix='.md', mode='w', encoding='utf-8')
        with open(input_path, 'r', encoding='utf-8') as txt_file:
            temp_md_file.write(txt_file.read())
        temp_md_file.close()
        input_path = Path(temp_md_file.name)
        logger.debug(f"Created temporary .md file: {temp_md_file.name}")
    
    logger.info(f"Processing document: {source_name}")
    
    # Load tokenizer
    logger.info(f"Loading tokenizer: {embedding_model}")
    try:
        hf_tokenizer = AutoTokenizer.from_pretrained(embedding_model)
        tokenizer = HuggingFaceTokenizer(
            tokenizer=hf_tokenizer,
            max_tokens=max_tokens
        )
        logger.info(f"Tokenizer loaded successfully (max_tokens={max_tokens})")
    except Exception as e:
        logger.error(f"Failed to load tokenizer: {e}", exc_info=True)
        if temp_file:
            try:
                Path(temp_file).unlink()
            except Exception:
                pass
        if temp_md_file:
            try:
                Path(temp_md_file.name).unlink()
            except Exception:
                pass
        return 0
    
    # Convert document using thread-local converter
    logger.info("Initializing document converter")
    try:
        converter = get_thread_local_converter(
            ocr_enabled=ocr_enabled,
            ocr_lang=ocr_lang
        )
        logger.info("Converting document to DoclingDocument")
        result = converter.convert(source=str(input_path))
        doc = result.document
        logger.info("Document converted successfully")
    except Exception as e:
        logger.error(f"Failed to convert document: {e}", exc_info=True)
        if temp_file:
            try:
                Path(temp_file).unlink()
            except Exception:
                pass
        if temp_md_file:
            try:
                Path(temp_md_file.name).unlink()
            except Exception:
                pass
        return 0
    
    # Initialize HybridChunker
    logger.info("Initializing HybridChunker with markdown table serialization")
    try:
        serializer_provider = MDTableSerializerProvider()
        hybrid_chunker = HybridChunker(
            tokenizer=tokenizer,
            merge_peers=True,
            serializer_provider=serializer_provider
        )
        logger.debug("HybridChunker initialized successfully")
    except Exception as e:
        logger.error(f"Failed to initialize chunker: {e}", exc_info=True)
        if temp_file:
            try:
                Path(temp_file).unlink()
            except Exception:
                pass
        if temp_md_file:
            try:
                Path(temp_md_file.name).unlink()
            except Exception:
                pass
        return 0
    
    # Chunk the document
    logger.info("Starting document chunking")
    chunks: List[Chunk] = []
    chunk_index = 0
    
    try:
        chunk_iter = hybrid_chunker.chunk(dl_doc=doc)
        
        for chunk_obj in chunk_iter:
            try:
                contextualized_text = hybrid_chunker.contextualize(chunk=chunk_obj)
                
                if not contextualized_text or len(contextualized_text.strip()) < 10:
                    logger.debug("Skipping empty chunk")
                    continue
                
                # Detect if chunk contains a table
                is_table = False
                if hasattr(chunk_obj, 'meta') and chunk_obj.meta:
                    if hasattr(chunk_obj.meta, 'doc_items'):
                        for item in chunk_obj.meta.doc_items:
                            if hasattr(item, 'label') and 'table' in str(item.label).lower():
                                is_table = True
                                break
                
                # Extract page number
                page_number = 1
                if hasattr(chunk_obj, 'meta') and chunk_obj.meta:
                    if hasattr(chunk_obj.meta, 'doc_items') and chunk_obj.meta.doc_items:
                        first_item = chunk_obj.meta.doc_items[0]
                        if hasattr(first_item, 'prov') and first_item.prov:
                            for prov_item in first_item.prov:
                                if hasattr(prov_item, 'page_no'):
                                    page_number = prov_item.page_no
                                    break
                
                # Extract section title
                section_title = "Unknown Section"
                if hasattr(chunk_obj, 'meta') and chunk_obj.meta:
                    if hasattr(chunk_obj.meta, 'headings') and chunk_obj.meta.headings:
                        try:
                            heading = chunk_obj.meta.headings[-1] if isinstance(chunk_obj.meta.headings, list) else str(chunk_obj.meta.headings)
                            section_title = str(heading)[:100]
                        except Exception:
                            logger.debug("Failed to extract section title")
                
                # Create chunk with metadata
                chunk = Chunk(
                    chunk_id=f"{source_name}_chunk_{chunk_index:04d}",
                    content=contextualized_text,
                    content_type='table' if is_table else 'text',
                    page_number=page_number,
                    section_title=section_title,
                    chunk_index=chunk_index,
                )
                chunks.append(chunk)
                chunk_index += 1
                
                if chunk_index % 10 == 0:
                    logger.info(f"Processed {chunk_index} chunks")
            
            except Exception as e:
                logger.warning(f"Failed to process chunk {chunk_index}: {e}")
                continue
        
        logger.info(f"Successfully generated {len(chunks)} chunks")
    
    except Exception as e:
        logger.error(f"Error during chunking: {e}", exc_info=True)
        if temp_file:
            try:
                Path(temp_file).unlink()
            except Exception:
                pass
        if temp_md_file:
            try:
                Path(temp_md_file.name).unlink()
            except Exception:
                pass
        return 0
    
    # Save chunks to JSON
    logger.info(f"Saving {len(chunks)} chunks to {output_json}")
    try:
        output_path = Path(output_json)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        
        json_data = [asdict(chunk) for chunk in chunks]
        
        with open(output_path, 'w', encoding='utf-8') as f:
            json.dump(json_data, f, indent=2, ensure_ascii=False)
        
        logger.info(f"Successfully saved chunks to {output_path}")
        
        # Clean up temporary files if they were created
        if temp_file:
            try:
                Path(temp_file).unlink()
                logger.debug(f"Cleaned up temporary file: {temp_file}")
            except Exception as cleanup_error:
                logger.warning(f"Failed to clean up temporary file: {cleanup_error}")
        
        if temp_md_file:
            try:
                Path(temp_md_file.name).unlink()
                logger.debug(f"Cleaned up temporary .md file: {temp_md_file.name}")
            except Exception as cleanup_error:
                logger.warning(f"Failed to clean up temporary .md file: {cleanup_error}")
        
        return len(chunks)
    
    except Exception as e:
        logger.error(f"Failed to save JSON: {e}", exc_info=True)
        
        # Clean up temporary files on error
        if temp_file:
            try:
                Path(temp_file).unlink()
            except Exception:
                pass
        
        if temp_md_file:
            try:
                Path(temp_md_file.name).unlink()
            except Exception:
                pass
        
        return 0


def get_document_files(input_path: str) -> List[Path]:
    """
    Get list of document files from input path.
    
    Args:
        input_path: Path to file or directory
        
    Returns:
        List of document file paths
    """
    path = Path(input_path)
    
    if path.is_file():
        # Single file
        if path.suffix.lower() in SUPPORTED_FORMATS:
            return [path]
        else:
            logger.warning(f"Unsupported file format: {path.suffix}")
            return []
    
    elif path.is_dir():
        # Directory of files
        files = []
        for file_path in path.rglob('*'):  # Recursively find all files
            if file_path.is_file() and file_path.suffix.lower() in SUPPORTED_FORMATS:
                files.append(file_path)
        
        if files:
            logger.info(f"Found {len(files)} document file(s) in {path}")
        else:
            logger.warning(f"No supported document files found in {path}")
        
        return sorted(files)
    
    else:
        logger.error(f"Path does not exist or is not a file/directory: {input_path}")
        return []


def process_input_output(
    input_arg: str,
    output_arg: str,
    embedding_model: str = "intfloat/multilingual-e5-base",
    max_tokens: int = 1024,
    ocr_enabled: bool = True,
    ocr_lang: List[str] = None,
) -> Dict[str, int]:
    """
    Process input (file or directory) and generate output (file or directory).
    
    Output behavior:
        - If output_arg is a file path (has extension): saves to that exact path
        - If output_arg is a directory: creates {input_filename}_chunks.json
        - For directory input: output must be a directory
    
    Args:
        input_arg: Input file or directory path
        output_arg: Output file path or directory path
        embedding_model: HuggingFace embedding model ID
        max_tokens: Maximum tokens per chunk
        ocr_enabled: Enable OCR for PDFs
        ocr_lang: OCR languages
        
    Returns:
        Dictionary mapping output files to chunk counts
    """
    
    input_path = Path(input_arg)
    output_path = Path(output_arg)
    
    # Get all input files
    files = get_document_files(input_arg)
    if not files:
        logger.error("No files to process")
        return {}
    
    results = {}
    
    # Single file input
    if input_path.is_file():
        logger.info(f"Processing single file: {input_path.name}")
        
        # Determine if output is a directory or file
        # It's a directory if: no extension, exists as dir, or ends with separator
        is_output_dir = (
            output_path.suffix == '' or 
            output_path.is_dir() or 
            str(output_arg).endswith(('/', '\\'))
        )
        
        if is_output_dir:
            # Output is a directory - create file with input name + _chunks.json
            output_path.mkdir(parents=True, exist_ok=True)
            output_file = output_path / f"{input_path.stem}_chunks.json"
            logger.debug(f"Output directory specified, creating: {output_file.name}")
        else:
            # Output is a file path - use it directly
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_file = output_path
            logger.debug(f"Output file specified: {output_file.name}")
        
        num_chunks = chunk_document(
            str(input_path),
            str(output_file),
            embedding_model=embedding_model,
            max_tokens=max_tokens,
            ocr_enabled=ocr_enabled,
            ocr_lang=ocr_lang
        )
        
        if num_chunks > 0:
            results[str(output_file)] = num_chunks
    
    # Directory input
    else:
        logger.info(f"Processing directory: {input_path} ({len(files)} files)")
        
        # Output must be a directory for batch processing
        output_path.mkdir(parents=True, exist_ok=True)
        
        total_chunks = 0
        
        for idx, input_file in enumerate(files, 1):
            logger.info(f"[{idx}/{len(files)}] Processing: {input_file.name}")
            
            # Create output filename: {input_filename}_chunks.json
            output_file = output_path / f"{input_file.stem}_chunks.json"
            
            num_chunks = chunk_document(
                str(input_file),
                str(output_file),
                embedding_model=embedding_model,
                max_tokens=max_tokens,
                ocr_enabled=ocr_enabled,
                ocr_lang=ocr_lang
            )
            
            if num_chunks > 0:
                results[str(output_file)] = num_chunks
                total_chunks += num_chunks
        
        logger.info(f"Completed processing {len(files)} files, total chunks: {total_chunks}")
    
    return results


if __name__ == "__main__":
    import sys
    
    if len(sys.argv) < 3:
        logger.error("Insufficient arguments provided")
        logger.info("Usage: python document_process.py <input_file_or_dir_or_url> <output_file_or_dir> [--no-ocr] [--ocr-lang en,hi]")
        sys.exit(1)
    
    input_arg = sys.argv[1]
    output_arg = sys.argv[2]
    
    # Parse optional arguments
    ocr_enabled = True
    ocr_lang = ["en"]
    
    if "--no-ocr" in sys.argv:
        ocr_enabled = False
        logger.info("OCR disabled")
    
    if "--ocr-lang" in sys.argv:
        idx = sys.argv.index("--ocr-lang")
        if idx + 1 < len(sys.argv):
            ocr_lang = sys.argv[idx + 1].split(",")
            logger.info(f"OCR languages: {ocr_lang}")
    
    logger.info("Starting chunking pipeline")
    logger.info(f"Input: {input_arg}")
    logger.info(f"Output: {output_arg}")
    logger.info(f"OCR enabled: {ocr_enabled}")
    logger.info(f"OCR languages: {ocr_lang}")
    
    results = process_input_output(
        input_arg, 
        output_arg,
        ocr_enabled=ocr_enabled,
        ocr_lang=ocr_lang
    )
    
    if results:
        total_chunks = sum(results.values())
        logger.info("CHUNKING COMPLETE")
        logger.info(f"Total chunks created: {total_chunks}")
        logger.info(f"Output files: {len(results)}")
        for output_file, chunks in results.items():
            logger.info(f"  - {output_file} ({chunks} chunks)")
        sys.exit(0)
    else:
        logger.error("Failed to create chunks")
        sys.exit(1)