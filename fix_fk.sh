#!/bin/bash
# Quick script to fix the foreign key constraint

echo "Fixing documents table foreign key constraint..."
psql -U postgres -d yourdb -f saas-api/migrations/fix_documents_foreign_key.sql

echo ""
echo "Done! You can now upload files."
