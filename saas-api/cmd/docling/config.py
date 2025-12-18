"""
Configuration for document processing and Weaviate connection
"""
import os
from pathlib import Path

# Weaviate Configuration
WEAVIATE_HOST = os.getenv("WEAVIATE_HOST", "10.10.6.13")
WEAVIATE_HTTP_PORT = int(os.getenv("WEAVIATE_HTTP_PORT", "7080"))
WEAVIATE_GRPC_PORT = int(os.getenv("WEAVIATE_GRPC_PORT", "50051"))

# OpenAI Configuration
OPENAI_API_KEY = os.getenv("OPENAI_API_KEY", "")

# Paths
BASE_DIR = Path(__file__).parent.parent
UPLOAD_DIR = BASE_DIR / "uploads"
PROCESSED_DIR = BASE_DIR / "processed"
DATA_FOLDER = BASE_DIR / "docling" / "data_folder"
OUTPUT_FOLDER = DATA_FOLDER / "output"

# Ensure directories exist
UPLOAD_DIR.mkdir(exist_ok=True)
PROCESSED_DIR.mkdir(exist_ok=True)
DATA_FOLDER.mkdir(exist_ok=True)
OUTPUT_FOLDER.mkdir(exist_ok=True, parents=True)

# Processing Configuration
DEFAULT_CHUNK_LIMIT = int(os.getenv("DEFAULT_CHUNK_LIMIT", "10"))
DEFAULT_ALPHA = float(os.getenv("DEFAULT_ALPHA", "0.5"))
BATCH_SIZE = 10

# File Upload Configuration
MAX_UPLOAD_SIZE = int(os.getenv("MAX_UPLOAD_SIZE", "100000000"))  # 100MB
ALLOWED_EXTENSIONS = [".pdf", ".docx", ".md"]
