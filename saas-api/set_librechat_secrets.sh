#!/bin/bash
# Set LibreChat JWT secrets for the proxy
# These values come from LibreChat's .env file

export LIBRE_JWT_SECRET="16f8c0ef4a5d391b26034086c628469d3f9f497f08163ab9b40137092f2909ef"
export LIBRE_JWT_REFRESH_SECRET="eaa5191f2914e30b9387fd84e254e4ba6fc51b4654968a9b0803b456a54b8418"

echo "✅ LibreChat JWT secrets have been set:"
echo "   LIBRE_JWT_SECRET is set"
echo "   LIBRE_JWT_REFRESH_SECRET is set"
echo ""
echo "Now you can run the proxy with these secrets."
