#!/bin/bash

# Nginx setup script for lostmedia.zacloth.com
# This script helps set up nginx configuration

set -e

echo "Setting up Nginx for lostmedia.zacloth.com..."

# Check if running as root
if [ "$EUID" -ne 0 ]; then 
    echo "Please run as root (use sudo)"
    exit 1
fi

# Install nginx if not installed
if ! command -v nginx &> /dev/null; then
    echo "Installing nginx..."
    apt-get update
    apt-get install -y nginx
fi

# Copy nginx configuration
echo "Copying nginx configuration..."
cp nginx.conf /etc/nginx/sites-available/lostmedia.zacloth.com

# Create symlink
if [ ! -L /etc/nginx/sites-enabled/lostmedia.zacloth.com ]; then
    ln -s /etc/nginx/sites-available/lostmedia.zacloth.com /etc/nginx/sites-enabled/
fi

# Remove default nginx site if exists
if [ -L /etc/nginx/sites-enabled/default ]; then
    rm /etc/nginx/sites-enabled/default
fi

# Test nginx configuration
echo "Testing nginx configuration..."
nginx -t

if [ $? -eq 0 ]; then
    echo "Nginx configuration is valid!"
    echo ""
    echo "Next steps:"
    echo "1. Install SSL certificate using certbot:"
    echo "   certbot --nginx -d lostmedia.zacloth.com"
    echo ""
    echo "2. Update SSL certificate paths in nginx.conf if needed"
    echo ""
    echo "3. Reload nginx:"
    echo "   systemctl reload nginx"
    echo ""
    echo "4. Make sure backend is running on port 5000"
    echo "5. Make sure frontend is running on port 3000"
else
    echo "Nginx configuration test failed! Please check the configuration."
    exit 1
fi
